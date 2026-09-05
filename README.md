# Interview

Listens to an interviewer, works out which utterance was a question, and streams a
short bulleted answer built from the live résumé at
<https://kazemisoroush.github.io/resume/resume.txt>.

One Lambda serves the page and answers the questions. There is no S3 bucket, no
CloudFront distribution and no API Gateway: the page is a single HTML file, so the
function returns it directly and the function URL is the public link. That also
means there is no unauthenticated origin sitting behind the sign-in.

## How it runs

| Piece | What it is |
|---|---|
| `index.html` | The whole frontend. Web Speech API for listening, no build step, no dependencies. |
| `lambda/index.mjs` | Serves the page on `GET`, answers on `POST`, shelling out to the `claude` CLI. |
| `lambda/Dockerfile` | `public.ecr.aws/lambda/nodejs:22` plus the `claude` CLI. Build context is the repo root. |
| `infra/` | CDK in Go. `InterviewStack` and `InterviewCICDStack`, including the Cognito pool. |
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
writes to it, so this is a manual step:

```sh
./set-secret.sh
```

That mints the token, writes it to the secret, and forces the function to drop its cached
copy. Run it again to rotate.

Then create the one account, since the pool has no self-signup:

```sh
aws cognito-idp admin-create-user --region us-east-1 \
  --user-pool-id "$(aws cloudformation describe-stacks --region us-east-1 \
      --stack-name InterviewStack \
      --query "Stacks[0].Outputs[?OutputKey=='UserPoolId'].OutputValue" --output text)" \
  --username you@example.com --user-attributes Name=email,Value=you@example.com Name=email_verified,Value=true
```

Cognito emails a temporary password and the hosted page walks through setting a real one.

Until the token is set, every answer fails. The gate fails closed on purpose.

## Signing in

The function URL is public, and it spends a personal Claude subscription. Without a
gate, anyone who finds the URL runs Claude as the account holder: a rate limit
problem, and a subscription-terms problem, since a Max plan covers one person.

So the handler verifies a Cognito token on every answer, and the pool has no
self-signup. Opening the app on a device that has never been signed in sends you
straight to the hosted sign-in page, which returns the token on the fragment; the page
keeps it in `localStorage` and strips it from the address bar. Nothing to carry between
devices, and nothing to rotate by hand.

The authorizer verifies inside the handler rather than sitting on an API Gateway the way
the book project does, because API Gateway buffers the response and buffering is the one
thing this app cannot afford.

A sign-in lasts a day. The implicit grant returns no refresh token, so that day is the
whole session; sign in again and it is another day. On GitHub Pages, where the browser
talks to the API directly, the same fragment trick carries an API key as `#key=sk-ant-...`.

## Local

```sh
node server.mjs
```

The page detects that it is not being served from GitHub Pages and talks to the local
backend, so no key and no sign-in are needed. `server.mjs` never injects the Cognito
config, and the page treats its absence as "this is local".
