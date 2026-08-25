// Answers interview questions with `claude -p`, so the app runs on the Claude Code
// subscription instead of API credits. Serves index.html from the same origin.
//
//   node server.mjs
//   tailscale serve --bg 8787      # -> https://<your-mac>.<tailnet>.ts.net
//
import { createServer } from 'http';
import { spawn } from 'child_process';
import { readFile } from 'fs/promises';
import { tmpdir } from 'os';

const PORT = 8787;
const CLAUDE = process.env.HOME + '/.local/bin/claude';
const RESUME_URL = 'https://kazemisoroush.github.io/resume/resume.txt';

const CLEAN_ENV = { ...process.env };
delete CLEAN_ENV.ANTHROPIC_API_KEY;
delete CLEAN_ENV.ANTHROPIC_AUTH_TOKEN;

const resume = await fetch(RESUME_URL).then(r => r.text());
const SYSTEM = `You are Soroush Kazemi in a live job interview. Answer in his voice, first person.

Rules:
- Short scannable bullet hints, not paragraphs. Max 5 bullets, one line each.
- Concrete: name the tech, the scale, the outcome. No filler, no preamble, no "great question".
- If it is a behavioural question, shape the bullets as situation / what I did / result.
- Only claim what the resume supports.
- Never use tools. Answer straight from the resume below.

His resume:
${resume}`;

createServer(async (req, res) => {
  if (req.method === 'POST' && req.url === '/ask') {
    const chunks = [];
    for await (const c of req) chunks.push(c);
    const { q } = JSON.parse(Buffer.concat(chunks));
    console.log('Q:', q);

    res.writeHead(200, { 'content-type': 'text/plain; charset=utf-8' });
    const p = spawn(CLAUDE, [
      '-p', q,
      '--system-prompt', SYSTEM,
      '--exclude-dynamic-system-prompt-sections',
      '--model', 'claude-opus-5',
      '--allowed-tools', '',
      '--strict-mcp-config', '--mcp-config', '{"mcpServers":{}}',  // connectors cost startup time
      '--output-format', 'stream-json', '--include-partial-messages', '--verbose'
    ], {
      // a set ANTHROPIC_API_KEY shadows the claude.ai login and bills credits instead.
      // it must be absent, not empty -- an empty value still wins its precedence slot.
      env: CLEAN_ENV,
      cwd: tmpdir(),                      // away from the repo, so no CLAUDE.md is loaded
      stdio: ['ignore', 'pipe', 'pipe']   // else it waits 3s for stdin
    });

    // forward just the text deltas, so the browser gets plain streaming prose
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
          res.write(inner.delta.text);
        if (ev.type === 'result' && ev.is_error) res.write('\n' + ev.result);
      }
    });
    p.stdout.on('end', () => res.end());
    p.stderr.on('data', d => process.stderr.write(d));
    p.on('error', e => res.end('server: ' + e.message));
    req.on('close', () => p.kill());   // next question supersedes this one
    return;
  }

  const html = await readFile(new URL('./index.html', import.meta.url));
  res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' }).end(html);
}).listen(PORT, () => console.log(`http://localhost:${PORT}`));
