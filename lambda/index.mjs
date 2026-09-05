// Serves the interview page and answers questions by shelling out to the `claude`
// CLI, streaming its text deltas straight to the browser.
//
// Runs on a Lambda Function URL in RESPONSE_STREAM invoke mode -- that mode is why
// this is Node and not Python, which cannot stream a Lambda response at all.
import { spawn } from 'child_process';
import { CognitoJwtVerifier } from 'aws-jwt-verify';
import { readFile } from 'fs/promises';
import { mkdir } from 'fs/promises';
import { tmpdir } from 'os';
import { SecretsManagerClient, GetSecretValueCommand } from '@aws-sdk/client-secrets-manager';

const RESUME_URL = 'https://kazemisoroush.github.io/resume/resume.txt';
const HOME = '/tmp/home';   // the only writable path on Lambda; the CLI wants a config dir

let booted;

// Cold-start work, done once and reused across warm invocations.
function boot() {
  return booted ??= (async () => {
    await mkdir(HOME, { recursive: true });

    // Secret is a JSON map of env-var-name -> value, same shape book uses. It is
    // populated by hand after deploy, so tolerate an empty or placeholder value.
    const arn = process.env.PROVIDER_SECRET_ARN;
    if (arn) {
      const res = await new SecretsManagerClient({}).send(new GetSecretValueCommand({ SecretId: arn }));
      try {
        Object.assign(process.env, JSON.parse(res.SecretString ?? '{}'));
      } catch {
        console.warn('provider secret is not JSON yet; leaving env untouched');
      }
    }

    const resume = await fetch(RESUME_URL).then(r => r.text());
    return {
      html: (await readFile(new URL('./index.html', import.meta.url), 'utf8')).replace(
        '</head>',
        `<script>window.SIGN_IN=${JSON.stringify({
          domain: process.env.COGNITO_DOMAIN,
          clientId: process.env.COGNITO_CLIENT_ID
        })}</script></head>`
      ),
      system: `You are Soroush Kazemi in a live job interview. Answer in his voice, first person.

You are given raw speech from the interviewer's microphone. Most of it is not a
question: small talk, their own war stories, thinking aloud, the tail of the last
answer. If the utterance is not a question or a prompt aimed at the candidate,
reply with exactly SKIP and nothing else.

Rules:
- Short scannable bullet hints, not paragraphs. Max 5 bullets, one line each.
- Concrete: name the tech, the scale, the outcome. No filler, no preamble, no "great question".
- If it is a behavioural question, shape the bullets as situation / what I did / result.
- Only claim what the resume supports.
- Never use tools. Answer straight from the resume below.

His resume:
${resume}`
    };
  })();
}

// Verifies the sign-in token's signature, issuer, audience and expiry against the pool.
// Built once per container: it caches the pool's public keys, so only the first request of a
// cold start pays the fetch.
const verifier = CognitoJwtVerifier.create({
  userPoolId: process.env.COGNITO_USER_POOL_ID,
  clientId: process.env.COGNITO_CLIENT_ID,
  tokenUse: 'id'
});

// Function URL headers arrive lowercased whatever the browser sent.
const signedIn = async event => {
  const header = event.headers?.authorization ?? '';
  const token = header.startsWith('Bearer ') ? header.slice(7) : '';
  if (!token) return false;
  try {
    await verifier.verify(token);
    return true;
  } catch {
    return false;   // expired, forged, or minted for another pool: all the same answer
  }
};

// Spawn the CLI and write only its text deltas to the response stream.
function answer(question, system, out) {
  return new Promise(resolve => {
    const p = spawn('claude', [
      '-p', question,
      '--system-prompt', system,
      '--exclude-dynamic-system-prompt-sections',
      '--model', 'claude-opus-5',
      '--allowed-tools', '',
      '--strict-mcp-config', '--mcp-config', '{"mcpServers":{}}',
      '--output-format', 'stream-json', '--include-partial-messages', '--verbose'
    ], {
      env: { ...process.env, HOME },
      cwd: tmpdir(),
      stdio: ['ignore', 'pipe', 'pipe']   // else the CLI waits on stdin
    });

    let buf = '';
    p.stdout.on('data', d => {
      buf += d;
      const lines = buf.split('\n');
      buf = lines.pop();
      for (const line of lines) {
        if (!line.trim()) continue;
        let ev;
        try { ev = JSON.parse(line); } catch { continue; }
        const inner = ev.type === 'stream_event' ? ev.event : null;
        if (inner?.type === 'content_block_delta' && inner.delta.type === 'text_delta')
          out.write(inner.delta.text);
        if (ev.type === 'result' && ev.is_error) out.write('\n' + ev.result);
      }
    });
    p.stderr.on('data', d => console.error(String(d)));
    p.on('error', e => { out.write('claude failed to start: ' + e.message); resolve(); });
    p.on('close', () => resolve());
  });
}

export const handler = awslambda.streamifyResponse(async (event, responseStream) => {
  const { html, system } = await boot();
  const method = event.requestContext?.http?.method ?? 'GET';

  const reply = (statusCode, contentType) => awslambda.HttpResponseStream.from(responseStream, {
    statusCode,
    headers: { 'content-type': contentType, 'cache-control': 'no-store' }
  });

  if (method !== 'POST') {
    const out = reply(200, 'text/html; charset=utf-8');
    out.write(html);
    return out.end();
  }

  let body = {};
  try {
    const raw = event.isBase64Encoded ? Buffer.from(event.body, 'base64').toString() : event.body;
    body = JSON.parse(raw ?? '{}');
  } catch { /* falls through to the 400 below */ }

  if (!await signedIn(event)) {
    const out = reply(401, 'text/plain; charset=utf-8');
    out.write('not signed in');
    return out.end();
  }
  if (!body.q) {
    const out = reply(400, 'text/plain; charset=utf-8');
    out.write('no question');
    return out.end();
  }

  const out = reply(200, 'text/plain; charset=utf-8');
  await answer(body.q, system, out);
  out.end();
});
