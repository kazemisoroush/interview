package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

// TestInterviewCICDStackTrustsMainBranchOnly checks the deploy role only trusts main.
func TestInterviewCICDStackTrustsMainBranchOnly(t *testing.T) {
	// Arrange
	defer jsii.Close()
	app := awscdk.NewApp(nil)
	env := &awscdk.Environment{
		Account: jsii.String("111111111111"),
		Region:  jsii.String("us-east-1"),
	}

	// Act
	stack := NewInterviewCICDStack(app, "TestCICD", &awscdk.StackProps{Env: env})
	template := assertions.Template_FromStack(stack, nil)

	// Assert: the account's OIDC provider is imported, not created here.
	template.ResourceCountIs(jsii.String("AWS::IAM::OIDCProvider"), jsii.Number(0))
	template.ResourceCountIs(jsii.String("AWS::IAM::Role"), jsii.Number(1))
	template.HasResourceProperties(jsii.String("AWS::IAM::Role"), map[string]any{
		"RoleName": deployRoleName,
		"AssumeRolePolicyDocument": map[string]any{
			"Statement": assertions.Match_ArrayWith(&[]any{
				assertions.Match_ObjectLike(&map[string]any{
					"Action": "sts:AssumeRoleWithWebIdentity",
					"Condition": map[string]any{
						"StringEquals": map[string]any{
							"token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
							"token.actions.githubusercontent.com:sub": "repo:kazemisoroush/interview:ref:refs/heads/main",
						},
					},
				}),
			}),
		},
	})
}

// TestInterviewCICDStackScopesDeployToBootstrapRoles checks the role only assumes cdk roles.
func TestInterviewCICDStackScopesDeployToBootstrapRoles(t *testing.T) {
	// Arrange
	defer jsii.Close()
	app := awscdk.NewApp(nil)
	env := &awscdk.Environment{
		Account: jsii.String("111111111111"),
		Region:  jsii.String("us-east-1"),
	}

	// Act
	stack := NewInterviewCICDStack(app, "TestCICD", &awscdk.StackProps{Env: env})
	template := assertions.Template_FromStack(stack, nil)

	// Assert
	template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]any{
		"PolicyDocument": map[string]any{
			"Statement": assertions.Match_ArrayWith(&[]any{
				assertions.Match_ObjectLike(&map[string]any{
					"Action":   "sts:AssumeRole",
					"Resource": "arn:aws:iam::111111111111:role/" + cdkBootstrapQualifier + "-*",
				}),
			}),
		},
	})
}
