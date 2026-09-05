#!/usr/bin/env bash
# One-time (and rotation) setup: mint a subscription token, write it into the secret, and
# force the function to pick it up.
#
#   ./set-secret.sh
#
# The token never reaches the shell history, the script's output, or any file. The
# forced cold start at the end is not optional: the handler caches the secret in
# `booted`, so a warm container keeps serving the old value and every request keeps
# coming back 401 long after the secret is correct.
set -euo pipefail

REGION=us-east-1
STACK=InterviewStack

out() { aws cloudformation describe-stacks --region "$REGION" --stack-name "$STACK" \
          --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text; }

SECRET=$(out ProviderSecretArn)
APP=$(out AppUrl)
FN=$(aws cloudformation describe-stack-resources --region "$REGION" --stack-name "$STACK" \
       --query 'StackResources[?ResourceType==`AWS::Lambda::Function`].PhysicalResourceId' --output text)

echo "==> claude setup-token (a browser will open; finish the login there)"
TOKEN=$(claude setup-token | grep -oE 'sk-ant-[A-Za-z0-9_-]+' | tail -1)
[ -n "$TOKEN" ] || { echo "no token found in the setup-token output" >&2; exit 1; }

aws secretsmanager put-secret-value --region "$REGION" --secret-id "$SECRET" \
  --secret-string "$(printf '{"CLAUDE_CODE_OAUTH_TOKEN":"%s"}' "$TOKEN")" \
  >/dev/null

# Any configuration change replaces the running containers, which is the only way to
# drop the cached secret without waiting for the old ones to age out.
#
# The new value is merged into the existing environment rather than written over it:
# --environment replaces the whole map, and the stack also puts the Cognito ids there, so
# spelling out one variable here would sign everybody out until the next deploy.
ENV_JSON=$(aws lambda get-function-configuration --region "$REGION" --function-name "$FN" \
  --query Environment --output json \
  | python3 -c 'import json,sys,time; e=json.load(sys.stdin); e["Variables"]["SECRET_SET_AT"]=str(int(time.time())); print(json.dumps(e))')

aws lambda update-function-configuration --region "$REGION" --function-name "$FN" \
  --environment "$ENV_JSON" >/dev/null
aws lambda wait function-updated --region "$REGION" --function-name "$FN"

echo
echo "open this and sign in:"
echo "$APP"
