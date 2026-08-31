# Interview

A live interview answer helper. It listens to the interviewer, cuts the microphone
stream into questions on silence, and streams short bulleted hints built from the
live résumé. One Lambda serves the page and answers the questions: `index.html` is
the whole frontend, `lambda/index.mjs` the whole backend, `infra/` the CDK in Go.

One metric: seconds from the interviewer finishing a question to the first bullet
on screen. A rule that costs latency has to earn it.

# Review

This file is the rulebook and the review bar. The automated reviewer in
[review.yml](.github/workflows/review.yml) reviews every pull request against
it, and it is the only standard the review judges against. Change the rules by
pull request.

## Review bar

CI already checks formatting, vet, lint, tests, and secrets. A review never
repeats a check CI runs.

A review requests changes only for one of these four, and it quotes the diff for
each blocker:

1. **correctness** — a logic bug, wrong result, race, unhandled error, or a broken edge case.
2. **security** — a leaked secret, missing authn/authz, injection, or unsafe input at a trust boundary.
3. **docs** — a claim in prose or a doc-block the code does not support, or a comment that misleads.
4. **abstraction** — an external integration called straight from business logic, an env var or flag read inline instead of through a config layer, a magic literal that should be a named constant, or a one-implementation abstraction that earns nothing.

Anything else is a note, not a blocker. Say it once in the body, then approve.
Taste that no rule here covers is not a reason to hold a pull request.

## Review comments

Every review body follows this format. It is the only source of the shape.

Open with one of three headings, and nothing else:

```md
**🤖 Claude review: approved**
**🤖 Claude review: approved with notes**
**🤖 Claude review: changes requested**
```

Then one sentence saying what was found. Then the findings, if there are any.

Severity has three levels and nothing else. A blocker is 🔴 or 🟠. A note is 🟡,
and a note never blocks a merge.

| Badge | Level | Meaning |
|---|---|---|
| 🔴 | Critical | Breaks the system or exposes it. Merging this causes harm. |
| 🟠 | Major | Wrong or misleading. Merging this leaves a defect behind. |
| 🟡 | Minor | Worth fixing, but safe to merge without it. |

Each finding takes this shape, with `Why.` and `Fix.` each one sentence:

```md
**🟠 Major · correctness · [`index.html:64`](https://github.com/kazemisoroush/interview/blob/<sha>/index.html#L64)**
verdict() matches startsWith("skip"), so a real answer opening "Skipping the migration" is dropped.

**Why.** The sentinel is swallowing genuine answers, and the failure is silent: no card, no error.
**Fix.** Match SKIP as a whole word with /^skip\b/.
```

Close every body with this line:

```md
<sub>Posted by Claude from the review workflow.</sub>
```

Rules for the body:

- Link every file with a full URL against the head commit, as above. A relative
  link resolves against the repository root and lands on a page that does not
  exist, because the comment renders on the pull request page.
- Support each finding with a quote from the diff or the code. A finding with no
  supporting quote is not posted.
- Keep each field to one sentence. A finding that needs a paragraph is really
  several findings, so split it.
- Never leave inline comments. An unresolved review thread blocks the pull
  request, so put every finding in the single review body.

# Code rules

- This app is one HTML file, one handler and one stack. Keep it that way: no build
  step, no framework, no dependency the platform already provides.
- No abstraction with one implementation. A wrapper earns its place only when a
  second caller or a real seam exists, not in advance.
- Prefer named constants over magic numbers, and say what the number means. `GAP`
  is a real-world knob, so its comment records what breaks on either side of it.
- Name things explicitly, never abbreviate a word to save characters.
- Do not keep history in comments while changing the code. Never reference a ticket
  number in the code.
- Never remove an existing inline comment unless asked to. Keep doc-blocks to one
  short sentence, and add one only to record a correctness or security subtlety.
- Every linter rule applies to all projects, not one.

# Timing and speech

- Every timing constant is a guess about a human until a real interview tunes it.
  Leave the knob named and commented, never inlined.
- The microphone hears both sides of the room. Any logic that joins utterances
  together must survive the candidate's own answer landing between two questions.

# JavaScript

- Vanilla, browser-native, no build. `SpeechRecognition`, `fetch` and
  `AbortController` are the platform; do not wrap them.
- Non-trivial pure logic carries an assertion under `?test`, runnable in the browser
  console and in node. No framework.
- Anything async that can be superseded must say what happens to the work it
  replaces. A half-drawn answer is either finished or removed, never left behind.

# Go (CDK)

- Divide tests into 3 parts separated with comments. // Arrange // Act // Assert.
- The cdk-nag gate in `cdk synth` is part of CI. A suppression states its reason.
- Trust policies and IAM conditions are never wildcarded where an exact value works.
