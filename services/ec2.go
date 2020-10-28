package services

import (
	"log"
	"sync"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

var wg sync.WaitGroup

// EC2Model aws sdk ec2  interface
type EC2Model struct {
	ec2iface.EC2API
	workers int
	filter  []*ec2.Filter
}

const (
	sessionError = "error creating ec2 session"
)

// NewEC2Session uses lambda execution role to create new EC2 Sessions
func NewEC2Session() (*EC2Model, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		log.Fatal(sessionError)
		return nil, err
	}
	svc := ec2.New(sess)
	return &EC2Model{
		EC2API:  svc,
		workers: 1, //default workers count
	}
}

func (m *EC2Model) setWorkers(count int) *EC2Model {
	m.workers = count
	return m
}

// DescribeEC2Instances takes an filter and retrieves list of ec2 instances
func (m *EC2Model) DescribeEC2Instances(filter *ec2.DescribeInstanceInput) ([]*ec2.Instance, error) {
	instances := []*ec2.Instance{}
	result, err := m.EC2API.DescribeInstances(filter)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			return nil, awsErr
		}
	}
	for _, rsvp := range result.Reservations {
		for _, instance := range rsvp.Instances {
			instances = append(instances, instance)
		}
	}
	return instances, nil
}

// GetInstanceIDs uses go routines to concurrently & efficiently pull instanceIDs from ec2 reservations
func (m *EC2Model) GetInstanceIDs(filter *ec2.DescribeInstancesInput, rgxMatch string, rgxTag string) ([]*string, error) {
	instanceIds := []*string{}
	input := make(chan *ec2.Instance)
	output := make(chan *string)
	for i := 0; i < m.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for instance := range input {
				ec2Tags := make(map[string]string, 0)
				for _, tag := range instance.Tags {
					ec2Tags[*tag.Key] = *tag.Value
				}
				match, err := regex.MatchString(rgxMatch, ec2Tags[rgxTag])
				if err != nil {
					log.Panic(err)
				}
				if match == true {
					output <- instance.InstanceId
				}
			}
		}()
	}
	result, err := m.EC2API.DescribeInstances(filter)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			return nil, awsErr
		}
	}
	for _, rsvp := range result.Reservations {
		for _, instance := range rsvp.Instances {
			input <- instance
		}
	}

	close(input)
	wg.Wait()
	close(output)
	for instance := range output {
		instanceIds = append(instanceIds, instance)
	}
	return instanceIDs, nil
}

// SetEC2Filter sets filter for methods
func (m *EC2Model) SetEC2Filter(filter map[string][]string) *EC2Model {
	for k := range filter {
		attributes := ec2.Filter{Name: aws.String(k), Values: aws.StringSlice(filter[k])}
		m.filter = append(m.filter, &attributes)
	}
	return m
}

// StartEC2Instances takes slice of type string and starts in prll
func (m *EC2Model) StartEC2Instances(instanceIDs []*string) (*ec2.StartInstancesOutput, error) {
	result, err := m.EC2API.StartInstances(&ec2.StartInstancesInput{
		InstanceIds: instanceIDs,
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			return nil, awsErr
		}
	}
	return result, nil
}

// StopEC2Instances takes sliceof type string and stops in prll
func (m *EC2Model) StopEC2Instances(instanceIDs []*string) (*ec2.StopInstancesOutput, error) {
	result, err := m.EC2API.StopInstances(&ec2.StopInstancesInput{
		InstanceIds: instanceIDs,
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			return nil, awsErr
		}
	}
	return result, nil
}
