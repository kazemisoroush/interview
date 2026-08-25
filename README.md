# Interview

Listens to an interviewer, works out which utterance was a question, and streams a
short bulleted answer built from the live résumé at
<https://kazemisoroush.github.io/resume/resume.txt>.

One Lambda serves the page and answers the questions. There is no S3 bucket, no
CloudFront distribution and no API Gateway: the page is a single HTML file, so the
function returns it directly and the function URL is the public link. That also
means there is no unauthenticated origin sitting behind the passphrase gate.

## How it runs

| Piece | What it is |
|---|---|
| `index.html` | The whole frontend. Web Speech API for listening, no build step, no dependencies. |
| `lambda/index.mjs` | Serves the page on `GET`, answers on `POST`, shelling out to the `claude` CLI. |
| `lambda/Dockerfile` | `public.ecr.aws/lambda/nodejs:22` plus the `claude` CLI. Build context is the repo root. |
| `infra/` | CDK in Go. `InterviewStack` and `InterviewCICDStack`. |
| `server.mjs` | The same backend for local use: `node server.mjs`, then <http://localhost:8787>. |

Node rather than Python because a Python Lambda cannot stream its response, and the
answers have to appear a bullet at a time rather than after a ten second pause.

Answers run on a Claude subscription token, not an API key, so there is no
per-question API billing.

## First deploy

These need admin credentials and a browser, so they are done by hand, once.

```sh
claude setup-token                            # mints CLAUDE_CODE_OAUTH_TOKEN
cdk bootstrap aws://116129308579/us-east-1    # only if the region is not bootstrapped
cd infra && cdk deploy InterviewCICDStack     # creates the GitHub OIDC deploy role
```

Then set the repository variables `AWS_DEPLOY_ROLE_ARN` (from the stack output) and
`AWS_REGION`, and merge to `main` — CI deploys `InterviewStack` from there on.

Finally, fill the secret. `cdk deploy` creates it empty and nothing in this repo ever
writes to it:

```sh
aws secretsmanager put-secret-value \
  --secret-id "$(aws cloudformation describe-stacks --stack-name InterviewStack \
      --query 'Stacks[0].Outputs[?OutputKey==`ProviderSecretArn`].OutputValue' --output text)" \
  --secret-string '{"CLAUDE_CODE_OAUTH_TOKEN":"...","PASSPHRASE":"..."}'
```

Until that is set, `PASSPHRASE` is empty and every request is rejected. The gate fails
closed on purpose.

## The passphrase

The function URL is public, and it spends a personal Claude subscription. Without a
gate, anyone who finds the URL runs Claude as the account holder — a rate limit
problem, and a subscription-terms problem, since a Max plan covers one person. The
passphrase keeps it genuinely single-user. Rotate it by writing a new secret value;
no redeploy is needed.

## Local

```sh
node server.mjs
```

The page detects that it is not being served from GitHub Pages and talks to the local
backend, so no key and no passphrase are needed.
