package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

// appURL stands in for the function URL the real stack passes as the OAuth callback.
const appURL = "https://example.lambda-url.us-east-1.on.aws/"

// signInTemplate builds a bare stack holding only the sign-in resources. The app stack is
// not used here on purpose: it carries a Docker image asset, and building that image would
// turn a property check into a container build.
func signInTemplate(t *testing.T) assertions.Template {
	t.Helper()
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestSignIn"), &awscdk.StackProps{
		Env: &awscdk.Environment{
			Account: jsii.String("111111111111"),
			Region:  jsii.String("us-east-1"),
		},
	})
	signIn(stack, jsii.String(appURL))
	return assertions.Template_FromStack(stack, nil)
}

// TestSignInRefusesSelfSignup checks the public function URL cannot be turned into an
// account factory. Self-signup is what separates one user from the whole internet.
func TestSignInRefusesSelfSignup(t *testing.T) {
	// Arrange
	defer jsii.Close()

	// Act
	template := signInTemplate(t)

	// Assert
	template.ResourceCountIs(jsii.String("AWS::Cognito::UserPool"), jsii.Number(1))
	template.HasResourceProperties(jsii.String("AWS::Cognito::UserPool"), map[string]any{
		"AdminCreateUserConfig": map[string]any{"AllowAdminCreateUserOnly": true},
	})
}

// TestSignInReturnsTheTokenToTheAppOnly checks the callback is the app itself, so a token
// minted by this pool cannot be redirected to somewhere else.
func TestSignInReturnsTheTokenToTheAppOnly(t *testing.T) {
	// Arrange
	defer jsii.Close()

	// Act
	template := signInTemplate(t)

	// Assert
	template.HasResourceProperties(jsii.String("AWS::Cognito::UserPoolClient"), map[string]any{
		"CallbackURLs":               []any{appURL},
		"AllowedOAuthFlows":          []any{"implicit"},
		"AllowedOAuthScopes":         []any{"openid"},
		"GenerateSecret":             false,
		"SupportedIdentityProviders": []any{"COGNITO"},
	})
}
