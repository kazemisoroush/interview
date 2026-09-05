package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
	"github.com/cdklabs/cdk-nag-go/cdknag/v2"
)

// suppressNag records the cdk-nag rules the interview app accepts by design, with the reason
// for each. The IAM5 wildcard on the deploy role
// is suppressed next to the role itself.
func suppressNag(stack awscdk.Stack) {
	cdknag.NagSuppressions_AddStackSuppressions(stack, &[]*cdknag.NagPackSuppression{
		{
			Id:     jsii.String("AwsSolutions-IAM4"),
			Reason: jsii.String("The Lambda uses the AWS managed basic execution role for CloudWatch Logs, the standard minimal logging policy."),
		},
		{
			Id:     jsii.String("AwsSolutions-SMG4"),
			Reason: jsii.String("Automatic secret rotation is deferred; the OAuth token is rotated by updating the secret by hand."),
		},
		{
			Id:     jsii.String("AwsSolutions-COG8"),
			Reason: jsii.String("The Cognito plus tier is deferred; it is a paid tier whose sign-in threat detection is aimed at a user base, and this pool has no self-signup and one account."),
		},
	}, jsii.Bool(true))
}
