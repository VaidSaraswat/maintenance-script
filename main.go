package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/olekukonko/tablewriter"
)

// Function prints changes in the current environment
func printChanges(changes []*route53.Change, environment string) {
	fmt.Println("Records have been changed in this environment: " + environment)
	fmt.Println("The following records have been changed: ")
	changeData := make([][]string, len(changes))
	for i := 0; i < len(changeData); i++ {
		changeData[i] = make([]string, 3)
	}
	for i := 0; i < len(changes); i++ {
		name := *changes[i].ResourceRecordSet.Name
		dnsName := *changes[i].ResourceRecordSet.AliasTarget.DNSName
		weight := *changes[i].ResourceRecordSet.Weight
		changeData[i] = []string{name, dnsName, strconv.FormatInt(weight, 10)}
	}
	// Create and render table to stdout
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Record", "Alias Target", "Weight"})
	table.SetRowLine(true)
	for i := 0; i < len(changeData); i++ {
		table.Append(changeData[i])
	}
	table.Render()
}

// Function prints potential records being changed in the current environment
func printRecords(records []*route53.ResourceRecordSet, environment string) {
	fmt.Println("You are going to make changes in this environment: " + environment)
	fmt.Println("The following records will be changed: ")
	recordData := make([][]string, len(records))
	for i := 0; i < len(recordData); i++ {
		recordData[i] = make([]string, 3)
	}
	for i := 0; i < len(records); i++ {
		name := *records[i].Name
		dnsName := *records[i].AliasTarget.DNSName
		weight := *records[i].Weight
		recordData[i] = []string{name, dnsName, strconv.FormatInt(weight, 10)}
	}
	// Create and render table to stdout
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Record", "Alias Target", "Weight"})
	table.SetRowLine(true)
	table.SetRowLine(true)
	for i := 0; i < len(recordData); i++ {
		table.Append(recordData[i])
	}
	table.Render()
}

// Function determines which HostedZoneID and domain to use given the profile
func determineContext(profile string) (string, string, error) {
	var domain string
	var hostedZoneID string
	var err error
	if profile == "dev" {
		domain = ".xyz."
		hostedZoneID = "HOSTED_ZONEID"
		err = nil
	} else if profile == "staging" {
		domain = ".yyz."
		hostedZoneID = "HOSTED_ZONEID"
		err = nil
	} else if profile == "production" {
		domain = ".zzz."
		hostedZoneID = "HOSTED_ZONEID"
		err = nil
	} else {
		err = errors.New("Invalid AWS profile")
	}
	return domain, hostedZoneID, err
}

// Function determine whether the name is a valid record name
func isValidRecord(recordName string, domain string) bool {
	switch recordName {
	case "RECORD_NAME" + domain:
		return true
	case "RECORD_NAME" + domain:
		return true
	case "RECORD_NAME" + domain:
		return true
	case "RECORD_NAME" + domain:
		return true
	case "RECORD_NAME" + domain:
		return true
	default:
		return false
	}
}

// Function constructs a struct of type ListResourceRecordSetsInput
func constructInput(hostedZoneID *string, startRecID *string, startRecName *string, startRecType *string) *route53.ListResourceRecordSetsInput {
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId:          hostedZoneID,
		StartRecordIdentifier: startRecID,
		StartRecordName:       startRecName,
		StartRecordType:       startRecType,
	}
	return input
}

// Function filters the given resource slice to a another slice of valid records
func filterRecords(resourceRecordSet []*route53.ResourceRecordSet, domain string) []*route53.ResourceRecordSet {
	var result []*route53.ResourceRecordSet
	for i := 0; i < len(resourceRecordSet); i++ {
		if *resourceRecordSet[i].Type == "A" && isValidRecord(*resourceRecordSet[i].Name, domain) {
			result = append(result, resourceRecordSet[i])
		}
	}
	return result
}

// Function takes a slice of records and generates a slice of changes
func makeChanges(records []*route53.ResourceRecordSet, domain string, mode string) []*route53.Change {
	var changes []*route53.Change
	for i := 0; i < len(records); i++ {
		var weight int64
		// Depending on the mode and dns name assign weight respectively
		if mode == "on" {
			if *records[i].AliasTarget.DNSName == ("RECORD_NAME" + domain) {
				weight = 100
			} else {
				weight = 0
			}
		} else {
			//If dns name starts with k8s-a, then set weight to 100, otherwise set to 0
			matched, _ := regexp.MatchString(`REGEX`, *records[i].AliasTarget.DNSName)
			if matched {
				weight = 100
			} else {
				weight = 0
			}
		}
		// Create change struct which will contain the updated weight
		change := &route53.Change{
			Action: aws.String("UPSERT"),
			ResourceRecordSet: &route53.ResourceRecordSet{
				AliasTarget: &route53.AliasTarget{
					DNSName:              records[i].AliasTarget.DNSName,
					EvaluateTargetHealth: records[i].AliasTarget.EvaluateTargetHealth,
					HostedZoneId:         records[i].AliasTarget.HostedZoneId,
				},
				Name:          records[i].Name,
				SetIdentifier: records[i].SetIdentifier,
				Type:          records[i].Type,
				Weight:        aws.Int64(weight),
			},
		}
		changes = append(changes, change)
	}
	return changes
}

func main() {
	// Define flags, the default mode and profile are set to off and invalid
	modePointer := flag.String("mode", "off", "string used to determine if maintenance mode is on/off")
	profilePointer := flag.String("profile", "invalid", "string used to determine which environment you want the changes to take place")
	// Parse the flags
	flag.Parse()
	// Assign domain and hostedZoneId depending on the profile
	domain, hostedZoneID, err := determineContext(*profilePointer)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	//  Specify profile for config and region for requests
	session, sessionError := session.NewSessionWithOptions(session.Options{
		Config:  aws.Config{Region: aws.String("us-east-2")},
		Profile: *profilePointer,
	})
	if sessionError != nil {
		fmt.Println(sessionError)
		os.Exit(1)
	}
	// Create Service with session
	svc := route53.New(session)
	resourceRecordSetInput := constructInput(&hostedZoneID, nil, nil, nil)
	// Each response object contains an isTruncated field set to true/false
	// Keep requesting records until they've been filtered accordingly
	isTruncated := true
	var validRecords []*route53.ResourceRecordSet
	for isTruncated {
		// Make request with input
		response, responseErr := svc.ListResourceRecordSets(resourceRecordSetInput)
		if responseErr != nil {
			fmt.Println(responseErr)
			os.Exit(1)
		} else {
			// filteredRecords will contain filtered records from the response
			filteredRecords := filterRecords(response.ResourceRecordSets, domain)
			// Ensure that the filteredRecords contains atleast one valid record and add it the validRecords slice
			if len(filteredRecords) > 0 {
				validRecords = append(validRecords, filteredRecords...)
			}
			// Construct the input for subsequent requests using the fields
			// NextRecordIdentifier, NextRecordName, NextRecordType from response
			// This tells the sdk where to start looking from, for the subsequent request
			resourceRecordSetInput = constructInput(&hostedZoneID, response.NextRecordIdentifier, response.NextRecordName, response.NextRecordType)
			isTruncated = *response.IsTruncated
		}

	}
	// Print potential changes
	printRecords(validRecords, *profilePointer)
	// Read user prompt
	fmt.Print("Do you wish to continue? (yes/no): ")
	var prompt string
	fmt.Scanln(&prompt)

	// If prompt is no then exit program, otherwise continue on
	if prompt != "yes" {
		fmt.Println("No changes made, goodbye")
		os.Exit(0)
	}
	// Get all the changes, this will be passed in the changeBatch object
	changes := makeChanges(validRecords, domain, *modePointer)
	changeResourceRecordSetsInput := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: changes,
		},
		HostedZoneId: aws.String(hostedZoneID),
	}
	// Make the change request
	_, changeReqErr := svc.ChangeResourceRecordSets(changeResourceRecordSetsInput)
	if changeReqErr != nil {
		aerr, _ := changeReqErr.(awserr.Error)
		fmt.Println(aerr.Code(), aerr.Error())
		os.Exit(1)
	}
	// Output changes
	fmt.Println("Changes made, it may take more than a minute for changes to propagate")
	printChanges(changes, *profilePointer)
}
