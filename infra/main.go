// CDK app that hosts the interview app: one streaming Lambda behind a function URL.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscognito"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecrassets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/cdklabs/cdk-nag-go/cdknag/v2"
)

// appImageFile is the Dockerfile for the app Lambda, built from the repo root so it can
// COPY the page next to the handler.
const appImageFile = "lambda/Dockerfile"

// envSecretArn tells the handler where to read the provider token.
const envSecretArn = "PROVIDER_SECRET_ARN"

// The handler verifies the sign-in token against these, and the page reads them back to
// know where to send someone who is not signed in yet.
const (
	envUserPoolID  = "COGNITO_USER_POOL_ID"
	envClientID    = "COGNITO_CLIENT_ID"
	envLoginDomain = "COGNITO_DOMAIN"
)

// signInValidityDays is how long a sign-in lasts. A day means one sign-in on the morning of
// an interview rather than one mid-answer, and a day is Cognito's ceiling for these tokens.
// ponytail: implicit grant has no refresh token, so this is the whole session length.
// Move to the authorization code flow with PKCE if a day ever proves too short.
const signInValidityDays = 1

// NewInterviewStack defines the secret, the app Lambda, and its streaming function URL.
func NewInterviewStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, props)

	// Nothing populates the value: the token is filled in by hand after the first deploy.
	secret := awssecretsmanager.NewSecret(stack, jsii.String("ProviderSecrets"), &awssecretsmanager.SecretProps{
		Description:   jsii.String("CLAUDE_CODE_OAUTH_TOKEN for the interview app."),
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	fn := awslambda.NewDockerImageFunction(stack, jsii.String("App"), &awslambda.DockerImageFunctionProps{
		Code: awslambda.DockerImageCode_FromImageAsset(jsii.String(".."), &awslambda.AssetImageCodeProps{
			File: jsii.String(appImageFile),
			// Pinned too, so a deploy from an x86 laptop cannot build an x86 image
			// for the arm64 function below.
			Platform: awsecrassets.Platform_LINUX_ARM64(),
		}),
		// Pinned, not defaulted: the image is built by whatever machine runs cdk deploy, so
		// the function's architecture has to match what the deploy runner produces (arm64).
		Architecture: awslambda.Architecture_ARM_64(),
		MemorySize:   jsii.Number(2048),
		Timeout:      awscdk.Duration_Seconds(jsii.Number(60)),
		Environment:  &map[string]*string{envSecretArn: secret.SecretArn()},
	})
	secret.GrantRead(fn, nil)

	// The function URL is the public entry point; the handler verifies a Cognito token on
	// every answer. RESPONSE_STREAM is the point of it: the handler streams tokens back as
	// they arrive rather than buffering the whole answer. This is also why the authorizer
	// cannot sit on an API Gateway the way the book project does: API Gateway buffers, and
	// a buffered answer arrives after the interviewer has moved on.
	url := fn.AddFunctionUrl(&awslambda.FunctionUrlOptions{
		AuthType:   awslambda.FunctionUrlAuthType_NONE,
		InvokeMode: awslambda.InvokeMode_RESPONSE_STREAM,
	})

	pool, client, domain := signIn(stack, url.Url())
	fn.AddEnvironment(jsii.String(envUserPoolID), pool.UserPoolId(), nil)
	fn.AddEnvironment(jsii.String(envClientID), client.UserPoolClientId(), nil)
	fn.AddEnvironment(jsii.String(envLoginDomain), domain.BaseUrl(nil), nil)

	awscdk.NewCfnOutput(stack, jsii.String("AppUrl"), &awscdk.CfnOutputProps{Value: url.Url()})
	awscdk.NewCfnOutput(stack, jsii.String("UserPoolId"), &awscdk.CfnOutputProps{Value: pool.UserPoolId()})
	awscdk.NewCfnOutput(stack, jsii.String("ProviderSecretArn"), &awscdk.CfnOutputProps{Value: secret.SecretArn()})

	suppressNag(stack)
	return stack
}

// signIn stands up the user pool the handler verifies against and the hosted page people
// actually sign in on. There is no self-signup: the one account is created by hand, which is
// what keeps a public function URL from spending a personal subscription.
func signIn(stack awscdk.Stack, appURL *string) (awscognito.UserPool, awscognito.IUserPoolClient, awscognito.UserPoolDomain) {
	pool := awscognito.NewUserPool(stack, jsii.String("Users"), &awscognito.UserPoolProps{
		SelfSignUpEnabled: jsii.Bool(false),
		SignInAliases:     &awscognito.SignInAliases{Email: jsii.Bool(true)},
		PasswordPolicy: &awscognito.PasswordPolicy{
			MinLength:        jsii.Number(12),
			RequireLowercase: jsii.Bool(true),
			RequireUppercase: jsii.Bool(true),
			RequireDigits:    jsii.Bool(true),
			RequireSymbols:   jsii.Bool(true),
		},
		AccountRecovery: awscognito.AccountRecovery_EMAIL_ONLY,
		RemovalPolicy:   awscdk.RemovalPolicy_DESTROY,
	})

	// Implicit grant: the hosted page hands the token straight back on the fragment, which the
	// page already knows how to read. The authorization code flow would need a token exchange
	// and a client secret store for one user, which is more moving parts than a day-long
	// session is worth.
	client := pool.AddClient(jsii.String("AppClient"), &awscognito.UserPoolClientOptions{
		GenerateSecret: jsii.Bool(false),
		OAuth: &awscognito.OAuthSettings{
			Flows:        &awscognito.OAuthFlows{ImplicitCodeGrant: jsii.Bool(true)},
			Scopes:       &[]awscognito.OAuthScope{awscognito.OAuthScope_OPENID()},
			CallbackUrls: &[]*string{appURL},
		},
		AccessTokenValidity: awscdk.Duration_Days(jsii.Number(signInValidityDays)),
		IdTokenValidity:     awscdk.Duration_Days(jsii.Number(signInValidityDays)),
	})

	// The prefix is global across every AWS account, so it carries this account's id.
	domain := pool.AddDomain(jsii.String("LoginDomain"), &awscognito.UserPoolDomainOptions{
		CognitoDomain: &awscognito.CognitoDomainOptions{
			DomainPrefix: jsii.String(fmt.Sprintf("interview-%s", *stack.Account())),
		},
	})

	return pool, client, domain
}

// gitHubOIDCHost is the GitHub Actions OIDC issuer host.
const gitHubOIDCHost = "token.actions.githubusercontent.com"

// gitHubAudience is the audience GitHub sets when requesting AWS credentials.
const gitHubAudience = "sts.amazonaws.com"

// deploySubjects pin the trust to a push on the main branch of the interview repo.
//
// Two spellings, because GitHub now stamps immutable numeric ids into the subject
// claim: it sends repo:kazemisoroush@8931595/interview@1346032883:ref:refs/heads/main
// rather than the documented repo:owner/name form. Both are listed rather than
// wildcarded, since a wildcard in the owner position would trust any account whose
// name merely starts the same way.
var deploySubjects = []any{
	"repo:kazemisoroush/interview:ref:refs/heads/main",
	"repo:kazemisoroush@8931595/interview@1346032883:ref:refs/heads/main",
}

// deployRoleName is the IAM role GitHub Actions assumes to deploy.
const deployRoleName = "interview-github-actions-deploy"

// cdkBootstrapQualifier is the name prefix of the CDK bootstrap roles in this account.
const cdkBootstrapQualifier = "cdk-hnb659fds"

// NewInterviewCICDStack defines the deploy role, trusting the account's shared GitHub OIDC
// provider. The provider is imported, not created: an account may hold only one provider per
// issuer, and the other projects in this account already created it.
func NewInterviewCICDStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, props)

	providerArn := fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", *stack.Account(), gitHubOIDCHost)
	provider := awsiam.OpenIdConnectProvider_FromOpenIdConnectProviderArn(
		stack, jsii.String("GitHubOIDC"), jsii.String(providerArn),
	)

	principal := awsiam.NewFederatedPrincipal(
		provider.OpenIdConnectProviderArn(),
		&map[string]any{
			"StringEquals": map[string]any{
				gitHubOIDCHost + ":aud": gitHubAudience,
				gitHubOIDCHost + ":sub": deploySubjects,
			},
		},
		jsii.String("sts:AssumeRoleWithWebIdentity"),
	)

	role := awsiam.NewRole(stack, jsii.String("GithubActionsDeploy"), &awsiam.RoleProps{
		RoleName:    jsii.String(deployRoleName),
		AssumedBy:   principal,
		Description: jsii.String("GitHub Actions assumes this via OIDC to deploy InterviewStack."),
	})

	bootstrapRoles := fmt.Sprintf("arn:aws:iam::%s:role/%s-*", *stack.Account(), cdkBootstrapQualifier)
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("sts:AssumeRole"),
		Resources: jsii.Strings(bootstrapRoles),
	}))

	cdknag.NagSuppressions_AddResourceSuppressions(role, &[]*cdknag.NagPackSuppression{
		{
			Id:     jsii.String("AwsSolutions-IAM5"),
			Reason: jsii.String(fmt.Sprintf("The deploy role may only assume the CDK bootstrap roles, which share the %s-* name prefix. The wildcard is scoped to those roles in this account.", cdkBootstrapQualifier)),
		},
	}, jsii.Bool(true))

	awscdk.NewCfnOutput(stack, jsii.String("DeployRoleArn"), &awscdk.CfnOutputProps{Value: role.RoleArn()})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	NewInterviewStack(app, "InterviewStack", &awscdk.StackProps{Env: stackEnv()})
	NewInterviewCICDStack(app, "InterviewCICDStack", &awscdk.StackProps{Env: stackEnv()})
	awscdk.Aspects_Of(app).Add(cdknag.NewAwsSolutionsChecks(&cdknag.NagPackProps{Verbose: jsii.Bool(true)}), nil)
	app.Synth(nil)
}

// stackEnv reads the deployment target from the standard CDK environment variables.
func stackEnv() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
		Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	}
}
