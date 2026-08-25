# Build context is the repo root, so this can COPY index.html.
FROM public.ecr.aws/lambda/nodejs:22

# The claude CLI is the whole point of this function.
RUN npm install -g @anthropic-ai/claude-code && npm cache clean --force

WORKDIR ${LAMBDA_TASK_ROOT}
COPY lambda/package.json ./
RUN npm install --omit=dev && npm cache clean --force
COPY lambda/index.mjs ./
COPY index.html ./

CMD ["index.handler"]
