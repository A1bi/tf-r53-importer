package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/manifoldco/promptui"
)

var zones = []string{
	"example.com",
}
var albNumber = 1

const terragruntPath = "/path/to/terragrunt"
const workingDir = "/path/to/workingdir"

var sess *session.Session
var client *route53.Route53

func main() {
	initialize()

	for _, zoneName := range zones {
		zone := findZone(&zoneName)
		if zone == nil {
			continue
		}

		zoneId := strings.Replace(*zone.Id, "/hostedzone/", "", 1)
		fmt.Printf("Zone %s found with ID: %s.\n", zoneName, zoneId)

		importZone(&zoneName, &zoneId)
		findAndImportRecord(zoneName, zoneId, "", "ns", "NS")
		findAndImportRecord(zoneName, zoneId, "", "main_aaaa", "AAAA")
		findAndImportRecord(zoneName, zoneId, "", "main_a", "A")
		findAndImportRecord(zoneName, zoneId, "www", "www_aaaa", "AAAA")
		findAndImportRecord(zoneName, zoneId, "www", "www_a", "A")
	}
}

func initialize() {
	sess = session.Must(session.NewSession())
	client = route53.New(sess)

	os.Chdir(fmt.Sprintf("%s/pages_%d", workingDir, albNumber))
}

func findZone(name *string) *route53.HostedZone {
	params := route53.ListHostedZonesByNameInput{
		DNSName:  name,
		MaxItems: aws.String("1"),
	}
	zones, error := client.ListHostedZonesByName(&params)

	if error != nil {
		fmt.Printf("%s\n", error)
		return nil
	}

	fullName := fmt.Sprintf("%s.", *name)
	if len(zones.HostedZones) < 1 || *zones.HostedZones[0].Name != fullName {
		showError(fmt.Sprintf("Zone %s not found.\n", *name))
		return nil
	}

	return zones.HostedZones[0]
}

func findAndImportRecord(zoneName, zoneId, subdomain, recordName, recordType string) {
	domain := zoneName
	if len(subdomain) > 0 {
		domain = fmt.Sprintf("%s.%s", subdomain, zoneName)
	}
	fullDomain := fmt.Sprintf("%s.", domain)

	records, error := findRecord(&zoneId, &domain, &recordType)

	if error != nil {
		showError(error)
		return
	}

	if len(records.ResourceRecordSets) < 1 || *records.ResourceRecordSets[0].Name != fullDomain {
		fmt.Printf("Found record: %s", records)
		showError(fmt.Sprintf("%s-Record not found for %s.", recordType, domain))
		return
	}

	recordId := fmt.Sprintf("%s_%s_%s", zoneId, domain, recordType)
	fmt.Printf("%s-Record found for %s with ID: %s.\n", recordType, zoneName, recordId)

	importRecord(&recordName, &zoneName, &recordId)
}

func findRecord(zoneId, name, recordType *string) (*route53.ListResourceRecordSetsOutput, error) {
	params := route53.ListResourceRecordSetsInput{
		HostedZoneId:    zoneId,
		StartRecordName: name,
		StartRecordType: recordType,
		MaxItems:        aws.String("1"),
	}
	return client.ListResourceRecordSets(&params)
}

func importZone(zoneName, zoneId *string) {
	address := fmt.Sprintf("aws_route53_zone.cms_zone[\"%s\"]", *zoneName)
	importResource(&address, zoneId)
}

func importRecord(recordName, zoneName, recordId *string) {
	address := fmt.Sprintf("aws_route53_record.cms_domain_%s[\"%s\"]", *recordName, *zoneName)
	importResource(&address, recordId)
}

func importResource(address, id *string) {
	fmt.Printf("$ terragrunt import %s %s\n", *address, *id)
	cmd := exec.Command(terragruntPath, "import", *address, *id)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	error := cmd.Run()
	if error != nil {
		prompt := promptui.Prompt{
			Label:     "Retry",
			IsConfirm: true,
		}

		_, err := prompt.Run()

		if err == nil {
			importResource(address, id)
		} else {
			showError("")
		}
	}
}

func showError(msg interface{}) {
	fmt.Printf("%s\n", msg)

	prompt := promptui.Prompt{
		Label:     "Continue",
		IsConfirm: true,
	}

	_, err := prompt.Run()

	if err != nil {
		os.Exit(1)
	}
}
