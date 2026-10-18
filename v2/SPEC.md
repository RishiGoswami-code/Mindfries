# Mindfries V2 --- Adaptive Engineering Assessment Platform

**Product specification · End-to-end workflow · Architecture ·
Evaluation and integrity model**

> **Product promise:** Real skills. Real evidence. Better hiring.\
> **Version:** V2\
> **Status:** Product blueprint for implementation\
> **Core principle:** Evaluate the candidate's engineering capability
> through multiple kinds of work---not through one interview, one IDE
> task, or one universal score.

------------------------------------------------------------------------

## 1. Executive summary

Mindfries V2 is an evidence-first engineering assessment platform. It
gives candidates a realistic workspace where they can demonstrate how
they research, design, implement, debug, test, review, explain, and
deliver software. It gives hiring teams a structured capability profile
backed by inspectable evidence rather than an opaque ranking.

The platform has three main parts:

1.  **Mindfries web app** --- recruiter configuration, candidate onboarding, assessment dashboard, and a unified floating workspace with tabs for the AI Help Assistant, System Design Editor, Research Lab, and other role-relevant tools. Candidates can open the tabs they need without leaving the main assessment view.
2.  **Mindfries local CLI and assessment agent** --- a command the
    candidate runs in their terminal. It checks the environment,
    configures the assessment workspace and Git, opens the candidate's
    preferred IDE, and records only the activity permitted by the
    assessment's disclosed policy. 
3.  **Mindfries backend** --- session orchestration, task delivery,
    evidence ingestion, rubric-based evaluation, adaptive follow-ups,
    integrity event classification, report generation, and recruiter
    review.

The assessment should adapt to a candidate's demonstrated strengths
while keeping the standards for each competency consistent. Someone who
is excellent at research or architecture should be able to show that
strength; someone who excels at implementation or debugging should have
a fair way to demonstrate it too. An untested skill is **unknown**, not
automatically weak.

The AI avatar is one source of evidence. It must **not** outweigh the
work itself. Code, design decisions, research quality, debugging, tests,
delivery, and technical reasoning remain central. The default scoring
model gives the avatar/interview a small, bounded contribution, and
recruiters can inspect the underlying evidence rather than relying on a
single number.

------------------------------------------------------------------------

## 2. V1 baseline and non-negotiable parallel-build strategy

### What exists today (V1)

V1 is already built and must be treated as a production product. Its known candidate-facing capabilities are:

- Candidate portal.
- Browser-based IDE.
- Candidate dashboard.
- AI chatbot.

Do not assume these V1 capabilities can be removed, replaced, or refactored as part of V2 work. The V1 codebase, user journey, deployed service, database behavior, and existing chatbot must continue to behave as they currently do.

### What V2 adds or changes

V2 evolves the assessment model beyond the current browser-only IDE and chatbot. Its planned additions include:

- A candidate-invoked terminal CLI and short-lived local assessment agent, without requiring an IDE extension.
- Opening the candidate’s supported local IDE and preparing a task workspace and Git workflow.
- A floating, policy-controlled AI Help Assistant with clear permitted actions.
- Explicit allowlists for developer platforms such as Supabase, GitHub, official documentation, package registries, and task-specific CI/deployment services.
- System Design Canvas, Research Lab, debugging/reliability, code review, testing, and delivery evidence.
- A shared evidence timeline and evidence-backed capability profile.
- Adaptive assessment routes that let candidates demonstrate different role-relevant strengths.
- Disclosed, bounded integrity monitoring and human review.
- Optional GitHub, CI, and deployment integrations, only when relevant to the role.

V2 may reuse stable V1 components only after their dependencies and contracts are understood. Reuse must not require a breaking change to V1. Prefer isolated V2 adapters or duplicated thin integration layers over risky shared refactors.

### The governing rule

**Build, test, and run V2 independently until the V2 acceptance criteria in this document pass. Only then perform a planned, reversible cutover.**

During the build period:
- V1 remains the default product and the only production candidate path unless an explicitly approved V2 pilot is configured separately.
- Do not redirect existing candidate URLs or sessions to V2.
- Do not replace V1's browser IDE or chatbot.
- Do not alter V1 behavior to accommodate V2.
- Do not run V2 database migrations against V1 tables unless the change is demonstrably backward-compatible, reviewed, and explicitly approved. The default is a separate V2 schema or database.
- Do not share mutable feature flags, session state, task data, credentials, queues, or deployment settings between V1 and V2.
- Do not run untested V2 background jobs against V1 data.
- Keep V1 rollback and recovery procedures intact.

### Required isolation boundaries

| Area | V1 protection requirement | V2 build approach |
|---|---|---|
| Source code | Existing V1 behavior must remain unchanged | Use a separate V2 branch/worktree or isolated V2 application/package; keep changes reviewable |
| URLs and routing | Existing candidate URLs keep serving V1 | Separate V2 routes/domain or explicit isolated feature path |
| Deployment | V1 deployment pipeline remains stable | Separate V2 service/project/environment and deployment pipeline |
| Environment variables/secrets | Never overwrite V1 settings | V2-specific environment variables and scoped credentials |
| Database | No destructive or unreviewed V1 schema/data changes | Prefer separate V2 database or schema; use V2-prefixed tables and versioned migrations |
| API | Existing V1 contracts stay compatible | V2 versioned API namespace, e.g. `/v2/...` |
| Background jobs/queues | V1 jobs and queues remain untouched | Separate V2 queues, workers, schedules, and idempotency namespace |
| Candidate sessions | V1 sessions must not be migrated or invalidated | Separate V2 session state and credentials |
| AI chatbot/policies | V1 chatbot continues unchanged | Implement the V2 policy-controlled assistant on the V2 path |
| Logs and analytics | Preserve existing V1 telemetry | Separate V2 event namespace and dashboards; avoid mixing evidence streams |
| Integrations | Existing V1 grants keep working | V2-scoped OAuth grants, tokens, and project permissions |
| Testing | V1 regression suite remains a release gate | Run V1 regression tests and V2 tests independently, then run coexistence tests |

### Safe implementation workflow

1. **Inventory V1 before coding.** Record the current app entry points, routes, deployment services, database tables/migrations, authentication, candidate session lifecycle, IDE implementation, chatbot integration, background jobs, and environment variables. Mark the V1 files/services that must not change.
2. **Create an isolated V2 development path.** Use a separate branch/worktree or V2 app/package, separate deployment target, separate V2 environment variables, and a separate V2 schema/database.
3. **Write contract tests for V1.** Capture critical journeys: candidate login, opening the portal, using the browser IDE, using the existing chatbot, dashboard loading, saving work, and submitting/completing an assessment where supported.
4. **Build V2 behind a separate route or domain.** Do not route ordinary V1 traffic to it. Do not introduce V2 code paths into V1 runtime unless the change is isolated and feature-disabled by default.
5. **Use synthetic or dedicated V2 test data.** Never use real candidate data for early testing unless explicitly authorized and protected.
6. **Test V2 components independently.** Test the CLI, agent, local IDE launch, Git, assistant policy gateway, developer-platform allowlists, canvas, research workspace, evidence capture, evaluators, and reports.
7. **Run coexistence tests.** Confirm V1 remains functional while V2 is deployed, running background tasks, writing its own data, and under test load.
8. **Run security and privacy review.** Validate scoped credentials, assistant tool restrictions, agent lifecycle, retention/deletion, candidate consent, and integrity classifications.
9. **Run a controlled V2 pilot.** Use a separate test cohort and explicit V2 enrollment; do not silently move existing V1 candidates.
10. **Approve cutover only after sign-off.** Product, engineering, security/privacy, and assessment-quality owners must accept the V2 release checklist.
11. **Cut over gradually and reversibly.** Use an explicit routing/configuration change with a documented rollback. Keep V1 deployable and its data intact during the agreed rollback window.
12. **Retire V1 only by a separate decision.** Do not delete V1 code, data, deployments, or routes as part of ordinary V2 launch work.

### Definition of “V1 undisturbed”

V1 is considered undisturbed only if:
- Existing candidate routes and authentication still work.
- Candidate portal, browser IDE, dashboard, and chatbot behavior remains unchanged.
- Existing data and sessions are not lost or invalidated.
- V1 deployment and environment settings are not overwritten.
- V1 background jobs and integrations remain operational.
- V1 regression tests pass before and after V2 deployments.
- V2 failures can be isolated and rolled back without restoring or repairing V1.
- V2 test data, telemetry, credentials, and migrations cannot accidentally mutate V1 resources.

## 3. Product goals

### Primary goals

-   Measure multiple engineering competencies with realistic,
    job-related tasks.
-   Give candidates a choice of legitimate approaches and room to
    demonstrate their strongest skills.
-   Keep the candidate's browser, local IDE, design canvas, research
    tools, and AI-avatar conversation connected to one assessment
    session.
-   Collect a transparent, relevant evidence timeline without claiming
    to observe everything on a personal computer.
-   Make evaluation traceable: every important conclusion should point
    to an artifact, event, test result, or recorded explanation.
-   Produce a capability profile with strengths, demonstrated gaps,
    confidence, and areas that were not assessed.
-   Help hiring teams make better-informed decisions while keeping the
    final decision with humans.
-   Support accessibility, reasonable accommodations, network
    interruptions, environment failures, and candidates who do not have
    a GitHub account.

### Non-goals

-   Claiming that any tool can detect every possible form of cheating or
    AI assistance.
-   Treating application focus changes, typing speed, accent, facial
    expression, confidence, or personality as proof of dishonesty or
    engineering ability.
-   Forcing every candidate into the same narrow implementation task.
-   Ranking candidates using a single unexplained score.
-   Making an AI avatar the main judge of a candidate.
-   Requiring deployment or a public GitHub account for every role.
-   Installing a permanent system-wide monitoring service for the MVP.

------------------------------------------------------------------------

## 4. Guiding principles

1.  **Evidence before inference.** Separate what was observed from what
    the system thinks it might mean.
2.  **Many ways to demonstrate skill.** Implementation is important, but
    so are research, architecture, debugging, testing, review, and
    delivery.
3.  **Comparable standards, flexible routes.** Candidates may take
    different assessment paths, but the rubric and difficulty
    expectations for each competency must remain consistent.
4.  **Unknown is not poor.** If a competency was not meaningfully
    tested, report it as "not assessed" or "insufficient evidence."
5.  **Human accountability.** AI can organize and summarize evidence,
    but hiring teams remain responsible for decisions.
6.  **Privacy by design.** Explain what is collected, why it is
    collected, who can see it, and how long it is retained.
7.  **Accessibility and recovery.** A broken dependency, unsupported
    operating system, network failure, or IDE crash must not silently
    become a negative candidate score.
8.  **AI use is policy-dependent.** Some assessments may restrict AI,
    some may allow approved tools, and some may evaluate AI-assisted
    engineering explicitly.
9.  **Candidate agency.** Candidates can pause the avatar, use text
    instead of voice where available, review submission status, and
    understand the assessment rules.
10. **No hidden "culture fit" scoring.** Assess job-related behaviors
    such as explaining trade-offs, responding constructively to review,
    and handling ambiguity---not similarity to a particular personality
    or background.

------------------------------------------------------------------------

## 5. Users and responsibilities

### Recruiter / hiring team

-   Defines the role, essential competencies, task options, AI-use
    policy, time window, accommodations, and evaluation rubric.
-   Reviews the capability profile and evidence.
-   Decides whether to progress, reject, or request a follow-up.

### Candidate

-   Reviews the task, rules, privacy notice, permitted tools, and
    submission requirements.
-   Chooses or follows the available assessment route.
-   Works in their preferred supported IDE and in the browser
    workspaces.
-   Explains decisions when useful, submits artifacts, and can report
    environment problems.

### Mindfries assessment orchestrator

-   Coordinates task state, workspace state, adaptive follow-ups, time,
    and evidence collection.
-   Makes sure all assessment routes produce evidence that can be
    evaluated against the intended rubric.

### AI avatar

-   Acts as a technically informed collaborator or challenger.
-   Asks contextual questions about decisions, assumptions, trade-offs,
    and evidence.
-   Contributes limited evidence about technical explanation and
    collaborative reasoning.
-   Does not decide the candidate's outcome or outweigh artifacts.

### Evaluators

-   Use rubric-grounded models and deterministic checks where
    appropriate.
-   Link every conclusion to supporting evidence and confidence.
-   Flag ambiguous cases for human review rather than inventing
    certainty.

### Hiring reviewer

-   Sees evidence, rubric scores, confidence, caveats, and integrity
    classifications.
-   Reviews flagged cases and makes the final hiring decision.

------------------------------------------------------------------------

## 6. End-to-end system architecture

``` text
                         RECRUITER / HIRING TEAM
                                  |
                                  v
                  Independent V1 and V2 product paths during build

  V1 (existing; protected)                         V2 (isolated build)
  ------------------------                         -------------------
  Candidate portal                                 V2 candidate portal/workspace
  Browser-based IDE                                Local CLI + assessment agent
  Existing dashboard                               Canvas / Research Lab / assistant
  Existing AI chatbot                              Evidence / adaptive evaluation
  Existing APIs + data                             Versioned V2 APIs + V2 data
  Existing deployment                              Separate V2 deployment
          |                                                 |
          v                                                 v
  V1 routes, config, database                      V2 routes, config, database
  remain unchanged                                  remain isolated
          |                                                 |
          +---------------- NO SHARED MUTATION -------------+
                                                            |
                                                            v
                                                 V2 Candidate Assessment UI
                                                            |
                                                            v
                                                   Role & Rubric Setup
                                  |
             +--------------------+----------------------+
             |                    |                      |
             v                    v                      v
      Floating Panels       System Design Canvas     Research Lab
      AI Avatar / Chat      diagrams + decisions     sources + notes
             |                    |                      |
             +--------------------+----------------------+
                                  |
                           Assessment API
                                  |
       +--------------------------+---------------------------+
       |                          |                           |
       v                          v                           v
 Session Orchestrator       Evidence Service           Evaluation Service
       |                          |                           |
       v                          v                           v
 Adaptive Task Engine      Event Timeline              Rubric Evaluators
       |                          |                           |
       +--------------------------+---------------------------+
                                  |
                           Evidence Store
                                  ^
                                  |
                         Mindfries Local CLI
                                  |
                    Local Assessment Agent (per session)
                                  |
                +-----------------+------------------+
                |                 |                  |
                v                 v                  v
          Workspace/Git      IDE Launcher      Local Evidence
          setup/verification  VS Code/other     within consent
                |
                v
          Candidate Codebase
                |
        Git / optional GitHub / optional CI or preview deployment
```

### Core services

-   **Session Service:** session creation, state transitions, expiry,
    pause/resume.
-   **Task Service:** task versions, task files, role-specific variants,
    hidden test policy.
-   **Local Agent Service:** short-lived session credentials, agent
    heartbeat, local setup status.
-   **Evidence Service:** artifact metadata, event timeline, source
    references, provenance, timestamps.
-   **Adaptive Assessment Service:** selects follow-up prompts or tasks
    based on missing or uncertain evidence.
-   **Evaluation Service:** deterministic checks, rubric-based model
    evaluation, evaluator version tracking.
-   **Integrity Service:** classifies observed events, verified policy
    violations, and review signals.
-   **Conversation Orchestrator:** AI avatar turn-taking, contextual
    retrieval, question selection, transcript handling.
-   **Reporting Service:** capability profile, evidence links, reviewer
    notes, decision history.
-   **Integration Service:** GitHub authorization, CI, optional
    deployment, email/notifications.

------------------------------------------------------------------------

## 7. Recruiter setup flow

### Step 1 --- Create an assessment blueprint

The recruiter specifies: - Role and seniority. - Essential competencies
versus differentiators. - Expected working context (greenfield, existing
codebase, legacy system, production incident, research-heavy task,
etc.). - Assessment duration and permitted pause rules. - Supported
operating systems and IDEs. - AI-use policy. - Submission
requirements. - Whether GitHub, CI, or deployment is relevant to the
role. - Accessibility and accommodation route. - Evidence retention and
reviewer access settings.

### Step 2 --- Choose competencies and task modes

A blueprint can include these modes:

  -----------------------------------------------------------------------
  Mode                    What it reveals         Typical evidence
  ----------------------- ----------------------- -----------------------
  Implementation          Can the candidate turn  Code diff, tests, build
                          requirements into       results, decisions
                          working software?       

  System design           Can they decompose a    Canvas, architecture
                          problem and reason      notes, alternatives,
                          about trade-offs?       constraints

  Research                Can they find, judge,   Sources, notes,
                          and synthesize useful   experiments,
                          information?            recommendations

  Debugging & reliability Can they investigate    Hypotheses, logs,
                          failures                reproduction, fix,
                          systematically?         verification

  Code comprehension &    Can they understand     Review comments, trace,
  review                  unfamiliar code and     refactor proposal
                          identify risks?         

  Testing & quality       Can they design         Test plan, tests,
                          meaningful checks and   coverage of
                          reason about edge       requirements, failure
                          cases?                  cases

  Delivery & workflow     Can they use Git and    Commit history, diff,
                          deliver a reproducible  run instructions,
                          result?                 optional CI/deployment

  Technical communication Can they explain        Contextual
                          decisions and           explanations,
                          collaborate             clarifying questions,
                          constructively?         response to challenge
  -----------------------------------------------------------------------

Not every role needs every mode. The recruiter chooses a meaningful
subset and marks essential competencies. Do not score candidates on
irrelevant capabilities merely because the platform supports them.

### Step 3 --- Configure the rubric

Each competency must have: - Observable criteria. - What strong,
adequate, weak, and insufficient evidence look like. - Which artifacts
can support the judgment. - How confidence is calculated or assigned. -
Which checks are deterministic and which require qualitative
evaluation. - Whether the criterion is essential or a differentiator. -
What to do if the evidence is missing or the environment fails.

### Step 4 --- Configure AI policy

Use one clearly stated policy per assessment:

-   **Restricted:** only the tools explicitly allowed by the task are
    permitted.
-   **Approved tools:** specified AI tools may be used; candidates
    should disclose relevant use and be able to explain submitted work.
-   **AI-assisted:** AI use is expected and evaluated as part of the
    workflow, including verification, review, and ownership.

The policy must be visible before the candidate begins. Do not advertise
a mode as "AI-proof."

### Step 5 --- Preview and validate

Before publishing, the recruiter can preview: - Candidate
instructions. - Task files. - Workspace options. - The rubric and
evidence requirements. - Time and pause behavior. - AI/avatar
settings. - The candidate report format. - Failure and accommodation
paths.

The assessment should be dry-run by someone other than the author to
catch unclear instructions, broken dependencies, hidden assumptions, and
unintended difficulty.

------------------------------------------------------------------------

## 8. Candidate onboarding

1.  Candidate opens a unique assessment link.
2.  Candidate reads the role-relevant task, time rules, AI policy,
    privacy notice, and data-retention summary.
3.  Candidate confirms consent for the specific collection described.
    Monitoring is not bundled into vague or unlimited consent.
4.  Candidate chooses supported IDE and available accessibility options.
5.  Mindfries checks whether the task requires GitHub, CI, or
    deployment. These are optional unless the role genuinely requires
    them.
6.  Candidate completes a brief environment preflight or starts the
    local setup command.
7.  Candidate sees the task overview, permitted tools, expected
    deliverables, available workspaces, and how to ask for help.
8.  The assessment begins only after required checks pass or the
    candidate chooses an explicitly supported alternative route.

Candidates must be able to understand what is being assessed and what is
being recorded. Where legally or operationally necessary, provide a
no-monitoring or alternate assessment route.

------------------------------------------------------------------------

## 9. Local CLI and assessment agent (no IDE extension)

### Product decision

V2 uses a terminal command to bootstrap the candidate's local
environment. It does **not** require an IDE extension. The command
downloads or invokes a signed, versioned CLI/agent, authenticates the
assessment session, prepares the workspace, configures Git, and opens
the candidate's preferred supported IDE.

The command must be specific to the candidate's session, short-lived,
and scoped. It must not embed a long-lived secret in shell history,
print reusable credentials, or grant unrestricted remote control over
the candidate's computer.

### Example candidate flow

``` text
1. Candidate clicks “Set up my workspace” in Mindfries.
2. Browser shows a session-specific command and clear setup instructions.
3. Candidate copies the command into their terminal and runs it.
4. CLI verifies the download/signature and authenticates the session.
5. CLI checks OS, Git, runtime/dependencies, disk space, and IDE availability.
6. Candidate confirms the target folder and any required installation or changes.
7. CLI downloads/clones the assessment repository or creates a clean workspace.
8. CLI creates a task branch, configures assessment-specific Git identity if needed,
   and writes task.md plus supporting files.
9. CLI opens the project in the candidate’s chosen supported IDE.
10. Local agent reports setup status and permitted evidence to Mindfries.
11. Candidate works locally while browser panels stay synchronized with the session.
12. Candidate reviews the diff and explicitly submits.
13. CLI verifies the submitted commit/artifacts and reports delivery status.
14. Local agent stops and removes temporary credentials/session state when finished.
```

### CLI responsibilities

-   Session authentication and expiry.
-   Environment preflight.
-   Repository/workspace setup.
-   Git availability and configuration.
-   IDE detection and launch.
-   Dependency installation only with clear notice and appropriate
    confirmation.
-   Task-file delivery and task-version verification.
-   Local agent startup and heartbeat.
-   Collection of permitted, assessment-relevant events.
-   Offline buffering and safe retry after a disconnect.
-   Git status/diff/commit verification.
-   Submission and cleanup.
-   A clear command or UI action to stop the assessment agent.

### Local agent responsibilities

-   Operate only for the active assessment session.
-   Collect only events authorized by the assessment policy.
-   Store a local event queue safely if the network disconnects.
-   Sign or otherwise protect event integrity where feasible.
-   Synchronize with the server using a scoped session credential.
-   Show agent status and last successful sync.
-   Stop when the session ends or the session expires.
-   Never silently become a persistent background service in the MVP.

### What the CLI cannot promise

A normal CLI process cannot reliably observe every action across every
application, browser, operating-system account, virtual machine, remote
desktop, or second device. It also cannot prove that a candidate did or
did not use every external AI tool. Stronger lockdown requires a
separate, explicitly disclosed managed environment and additional
technical controls. Mindfries must state the actual coverage of each
mode, not imply universal surveillance.

### Environment failure handling

  -----------------------------------------------------------------------
  Failure                             Expected response
  ----------------------------------- -----------------------------------
  Git missing                         Offer an approved installation path
                                      or supported alternative; do not
                                      score the candidate down

  Preferred IDE missing               Offer another supported IDE or a
                                      browser-based fallback if available

  Dependency install fails            Show actionable logs, retry, or
                                      provide a standardized
                                      container/workspace

  Repository cannot clone             Retry, check permissions/network,
                                      or provide a platform-hosted copy

  Network disconnects                 Queue permitted events locally,
                                      mark sync state, and resume safely

  IDE crashes                         Preserve workspace and Git state;
                                      allow restart

  Candidate has no GitHub account     Use platform submission or a
                                      private temporary repository

  OS unsupported                      Offer a documented alternative
                                      route before the timed assessment

  Candidate needs accommodation       Apply an approved route without
                                      lowering the competency standard

  Local agent stops unexpectedly      Notify candidate, preserve
                                      artifacts, pause or mark evidence
                                      gap; do not infer misconduct
  -----------------------------------------------------------------------

------------------------------------------------------------------------

## 10. The unified candidate workspace

The browser remains the central assessment hub while the candidate works in their actual IDE. One floating, draggable, minimizable panel contains tabs for the AI Help Assistant, System Design Editor, Research Lab, and other tools enabled for that assessment.

### Main workspace and floating tabbed panel

The candidate keeps the assessment page and their coding environment as the main work surfaces. A **single floating panel** provides tabs for the tools needed during the assessment. It is not a separate chatbot window with unrelated tools elsewhere on the page.

Suggested tabs include:

- **AI Help:** the policy-controlled Mindfries assistant for explanations, debugging, code review, tests, and task-related guidance.
- **System Design:** a canvas for architecture diagrams, components, data flows, alternatives, constraints, and trade-offs.
- **Research:** source notes, research questions, comparison matrices, hypotheses, experiments, and recommendations.
- **Debugging:** a focused workspace for hypotheses, log snippets, reproduction steps, and verification notes when relevant.
- **Notes / Evidence:** candidate-owned notes, decisions, links to artifacts, and a view of required deliverables.
- **Other role-specific tools:** optional tabs selected by the recruiter’s assessment blueprint, such as API design, testing strategy, or code review.

The panel should preserve each tab’s state when the candidate switches tabs, support resizing and minimizing, and avoid blocking the IDE or task content. The recruiter’s blueprint determines which tabs are available; candidates should not be forced to use every tab. When reopened, the panel should restore the last active tab.

All tabs belong to the same assessment session and evidence model. For example, a candidate can ask the AI Help tab about a trade-off, document alternatives in System Design, and add supporting sources in Research. Relevant artifacts can be linked together under the configured policy; private notes should not automatically become submitted evidence without clear notice.

The workspace also includes:

- **Assessment header:** task, timer/pause status, current mode, connection status, submission status.
- **Task brief:** requirements, constraints, acceptance criteria, permitted tools.
- **Activity timeline:** key events and artifact milestones visible to the candidate where appropriate.
- **Evidence/Deliverables panel:** required artifacts, checks, Git status, and submission progress.
- **Help and accessibility:** task clarification, technical-issue reporting, text alternatives, and supported accommodations.

The IDE remains the candidate’s primary coding surface. Mindfries should not force a custom editor on candidates who are more effective in a supported local IDE. The floating tabbed panel complements that IDE; it does not replace it.

### Workspace modes

#### A. Code & Execution

-   Read and modify an existing codebase or starter repository.
-   Run the application and tests.
-   Implement a feature, fix, or scoped task.
-   Capture task-relevant diffs, test results, build output, and
    milestones.
-   Support incremental submission and recovery.

#### B. System Design Canvas

-   Create architecture diagrams, components, flows, APIs, data models,
    and failure paths.
-   Record constraints and assumptions.
-   Compare alternatives and explain trade-offs.
-   Link design decisions to implementation or test evidence.
-   Evaluate the reasoning behind a diagram, not its visual polish.

#### C. Research Lab

-   Capture research questions and hypotheses.
-   Record sources and why they are credible/relevant.
-   Compare approaches in a structured matrix.
-   Record experiments, observations, benchmarks, and limitations.
-   Produce a recommendation with assumptions and uncertainty.
-   Reward source quality, synthesis, and decision quality---not number
    of tabs or links.

#### D. Debugging & Reliability

-   Present a reproducible bug, logs, failing tests, metrics, traces, or
    a simulated incident.
-   Ask the candidate to establish a baseline, form hypotheses,
    investigate, and verify.
-   Assess root-cause reasoning, safe fixes, regression protection, and
    communication.
-   Use a sandbox or synthetic incident; never expose real customer
    secrets or production data.

#### E. Code Comprehension & Review

-   Ask the candidate to trace unfamiliar code and explain its behavior.
-   Identify correctness, performance, security, maintainability, or
    testability concerns.
-   Prioritize findings by impact and evidence.
-   Allow a review-only route where changing code is not the intended
    competency.

#### F. Testing & Quality

-   Translate requirements into tests.
-   Consider edge cases, negative paths, invariants, and failure modes.
-   Evaluate whether tests would catch realistic regressions.
-   Separate test quality from raw line coverage.

#### G. Review & Explanation

-   Candidate summarizes their approach and limitations.
-   The avatar or reviewer can challenge a specific decision.
-   Candidate may correct the system, disagree, or ask for
    clarification.
-   A well-supported disagreement is positive evidence, not
    noncompliance.

------------------------------------------------------------------------

## 11. Adaptive assessment: candidates can be strong in different ways

### Core structure

Use a **common core + evidence-driven route + targeted follow-up**
model.

1.  **Common core:** a short, role-relevant baseline that establishes
    shared context and a minimum comparable evidence set.
2.  **Evidence-driven route:** the system chooses or offers a suitable
    mode based on the role blueprint and early evidence. A candidate who
    shows strong research skills can demonstrate them; a candidate who
    shows strong debugging skills can pursue that route.
3.  **Targeted follow-up:** ask a short follow-up only where evidence is
    missing, conflicting, or uncertain and the competency matters to the
    role.
4.  **Evidence sufficiency check:** stop when there is enough reliable
    evidence, not when every possible module has been completed.
5.  **Capability profile:** report each competency separately with
    evidence and confidence.

### Example routes

-   **Research-oriented engineer:** common core → research investigation
    → compare approaches → small validation or prototype → explanation
    of recommendation.
-   **Systems-oriented engineer:** common core → system-design canvas →
    failure/scaling trade-offs → targeted implementation or
    API/data-model follow-up.
-   **Implementation-oriented engineer:** common core → feature/fix →
    tests → code review → delivery verification.
-   **Debugging-oriented engineer:** common core → failing test or
    incident → investigation → root-cause hypothesis → fix or mitigation
    → regression test.
-   **Quality/reliability-oriented engineer:** common core → test
    strategy → fault analysis → reliability improvement → review of
    risks.
-   **Full-stack/generalist role:** a balanced route across
    implementation, design, testing, debugging, and delivery.

These are not permanent candidate labels. They are assessment routes for
gathering evidence. A candidate should not be categorized as "only a
researcher" or "only an implementer" based on one session.

### Fairness guardrails

-   Keep the rubric for each competency stable across candidates.
-   Track which competencies were assessed, how much evidence exists,
    and the difficulty of the tasks used.
-   Do not make an adaptive route easier simply because a candidate is
    struggling.
-   Do not make a route harder simply because a candidate performs well.
-   Use targeted follow-ups to close evidence gaps, not to punish or
    trap candidates.
-   Allow candidates to choose among equivalent task variants when the
    role permits it.
-   Calibrate task variants against the same rubric and expected level.
-   Report "not assessed" separately from "below expected level."
-   Check whether task paths create systematic differences across
    candidate groups.
-   Keep a record of why the system selected a follow-up route.

------------------------------------------------------------------------

## 12. AI avatar: important, but not the main judge

### Purpose

The avatar is a technically informed colleague who helps reveal how a
candidate reasons about work. It can ask for clarification, challenge
assumptions, suggest a counterargument, or ask the candidate to explain
evidence. It is not a generic chatbot and should not dominate the
assessment.

### Conversation behaviors

-   **Curious colleague:** "What constraint led you to choose this
    approach?"
-   **Technical challenger:** "What might fail if traffic increases
    tenfold?"
-   **Research partner:** "Which source or experiment most strongly
    supports that conclusion?"
-   **Design reviewer:** "What alternative did you reject, and why?"
-   **Reflective teammate:** "What would you improve with another hour?"
-   **Ownership check:** "Can you walk me through how this function
    behaves?"
-   **Constructive disagreement:** if the avatar's assumption is wrong,
    the candidate can explain why and point to evidence.

### Conversation loop

``` text
Candidate works in IDE / Canvas / Research Lab
                 |
                 v
New artifact, decision, test result, or relevant milestone
                 |
                 v
Conversation Orchestrator retrieves only relevant context
                 |
                 v
Avatar asks one focused question when useful
                 |
                 v
Candidate answers by voice or text, or defers
                 |
                 v
Answer is linked to the relevant artifact and rubric
                 |
                 v
Assessment continues; avatar does not interrupt constantly
```

### Avatar controls

-   Candidate can mute, pause, or defer a question.
-   Voice should have a text alternative where possible.
-   Provide clear indication when audio is being captured and when
    recording stops.
-   Avoid interrupting deep work; ask at natural milestones or when a
    clarification matters.
-   Make the avatar's role and limitations explicit.
-   Keep a record of the prompt/question and relevant answer for review.
-   Provide a way to flag an incorrect, confusing, or unfair avatar
    question.

### What the avatar may contribute evidence about

-   Clarity of technical explanations.
-   Ability to state assumptions and trade-offs.
-   Response to evidence-based challenge.
-   Ability to ask useful clarifying questions.
-   Technical ownership of submitted work.
-   Collaborative reasoning and willingness to revise a claim when
    evidence changes.

### What the avatar must not decide

-   It must not determine the overall outcome.
-   It must not dominate the score or compensate for weak technical
    evidence.
-   It must not infer engineering skill from accent, vocal pitch,
    fluency alone, facial expression, charisma, or perceived
    personality.
-   It must not use vague "culture fit" judgments.
-   It must not treat disagreement as a negative signal by itself.
-   It must not infer cheating solely because an answer is polished, a
    candidate pauses, or a candidate uses a permitted tool.

------------------------------------------------------------------------

## 13. Floating AI Help Assistant and tool permissions

### Product behavior

The floating assistant is the candidate’s permitted AI helper during the assessment. It should be useful for understanding the task, explaining concepts, interpreting errors, debugging, discussing alternatives, writing or reviewing code where the policy allows it, and navigating approved developer tools. It must follow the session’s policy rather than acting as an unrestricted general-purpose agent.

The assistant should:
- Be available from a draggable, minimizable floating panel in the Mindfries browser workspace.
- Receive only the task context and candidate artifacts needed to answer the current request.
- Explain errors and concepts, help debug, suggest tests, and review candidate work within the permitted policy.
- Clearly distinguish hints, explanations, and generated code when useful.
- Refuse disallowed requests with a brief explanation and, when possible, redirect to an allowed kind of help.
- Never secretly perform a prohibited action through a tool, browser, API, or external integration.
- Never expose hidden tests, answer keys, private evaluator instructions, secrets, or another candidate’s data.
- Keep a relevant, disclosed record of assistant interactions according to the session policy.
- Let the candidate pause, minimize, or close the panel without losing their work.
- Avoid interrupting the candidate with unsolicited messages.

### Separate AI-tool policy from developer-platform access

Mindfries must maintain two separate configuration lists:

1. **AI tools policy:** which AI assistants/models are permitted for the assessment.
2. **Developer platform allowlist:** which non-AI or task-required services the candidate may use.

Example configuration:

| Category | Example | Default handling |
|---|---|---|
| Built-in Mindfries AI Help Assistant | Floating assistant | Allowed if enabled by the assessment |
| External AI chat websites | General-purpose AI chat services | Restricted unless explicitly allowlisted |
| Local AI applications/models | Locally installed models | Restricted only where the chosen managed environment can enforce this |
| Supabase | Database, authentication, storage, project console | Allow if needed for the task |
| GitHub | Repository, pull request, commit workflow | Allow only with the configured access scope |
| Official documentation | Framework, language, API documentation | Allow by default if the recruiter chooses |
| Package registries | npm, PyPI, or other relevant registries | Allow as required for dependency installation |
| Cloud/CI/deployment console | Task-specific provider | Allow only when needed and scoped |
| Unrelated sites/services | Personal email, messaging, unrelated cloud accounts | Restrict where supported by the declared assessment environment |

This table is illustrative. Recruiters must be able to configure the exact allowlist by task and role. A platform is not automatically allowed merely because it is a developer tool; the assessment blueprint controls the policy.

### Assistant policy gateway

All assistant tool calls must pass through a backend policy gateway:

```text
Candidate asks floating assistant for help
                  |
                  v
        Session policy + task context
                  |
                  v
        Assistant generates response
                  |
                  v
      Does it request a tool/action?
             /             \
           No               Yes
           |                 |
           v                 v
   Policy check on      Policy gateway checks
   response content     tool, domain, scope,
           |             permissions, session
           |                 |
           |          +------+------+
           |          |             |
           |       Allowed       Blocked
           |          |             |
           |          v             v
           |     Execute with    Refuse safely;
           |     least privilege log policy reason
           |          |
           +----------+
                  |
                  v
      Return response and relevant evidence
```

The policy gateway must check tool name, destination/domain, action, session permissions, data scope, and credential scope. Do not rely only on a system prompt saying “do not use this tool.” Use server-side authorization, allowlisted integrations, scoped credentials, and audit logs.

### Example allowed requests

- “Explain this stack trace and suggest how I can isolate the bug.”
- “What does this Supabase error mean?”
- “Help me compare these two database schema designs.”
- “Review this function for edge cases.”
- “Suggest tests for the requirement I’m implementing.”
- “Explain the trade-offs between these approaches.”
- “Help me understand how to use the approved Supabase project for this task.”

Whether the assistant may generate a full solution, edit files, run commands, or call external services depends on the recruiter’s policy. The default should be **advice and explanations, with candidate-controlled changes**, unless the assessment explicitly evaluates agentic coding.

### Example blocked requests

- Requests to access a prohibited external AI assistant through the Mindfries tool gateway.
- Requests to retrieve hidden tests, answer keys, or confidential evaluator prompts.
- Requests to read unrelated private files or credentials.
- Requests to use an unapproved integration or exfiltrate code to an unapproved service.
- Requests to bypass the assessment’s configured policy or monitoring controls.

The assistant should not shame the candidate or automatically treat a blocked request as cheating. Record a policy event only when the event is relevant and disclosed; route ambiguous cases to review.

### Enforcement modes and honest limitations

- **Assistant-level enforcement:** reliable for actions and integrations routed through the Mindfries backend.
- **Browser-level restrictions:** may restrict websites in a controlled browser, but do not control every other browser or application on a personal device.
- **Managed assessment environment:** can enforce stronger domain/application restrictions through configured OS, browser, network, or virtual-desktop controls, subject to the environment’s actual capabilities.
- **Standard local CLI:** can prepare the workspace and collect permitted local evidence, but cannot guarantee that external AI websites, other desktop applications, local models, a second browser, or another device are unavailable.

The candidate must be told which enforcement mode is active. Do not present a restricted-assistant policy as proof that all external AI use has been prevented.

### Developer-platform security

When services such as Supabase are allowed:
- Use assessment-specific projects or least-privilege credentials whenever possible.
- Never ask candidates to paste personal passwords or long-lived API keys into the assistant.
- Provide mock/test credentials and safe sample data.
- Scope credentials to the minimum required resources and expire or revoke them after the session.
- Keep secrets out of prompts, logs, screenshots, and evidence exports.
- Do not give the assistant unrestricted production access.
- Record which integration was invoked and the scope of the action, not unrelated data from the platform.
- Make clear whether the candidate is using a sandbox, a shared test project, or a real external account.

## 14. Evaluation and weighting model

### Non-negotiable scoring principle

**The interviewer/avatar is a supporting dimension, not the center of
the assessment.** The candidate's actual engineering artifacts and
demonstrated problem-solving should carry most of the evaluation weight.

A suggested default for a broad software engineering assessment is shown
below. These values are starting points for calibration, not universal
truth. Recruiters may adjust them to reflect the role, but must document
why and keep the same rubric for candidates assessed for the same role.

  ------------------------------------------------------------------------
  Dimension              Illustrative default weight Main evidence
  --------------------- ---------------------------- ---------------------
  Implementation & code                          20% Code diff, behavior,
  correctness                                        tests, build/run
                                                     results

  System design &                                15% Canvas, architecture
  trade-off reasoning                                decisions,
                                                     constraints, failure
                                                     paths

  Debugging &                                    15% Investigation trail,
  root-cause reasoning                               hypotheses,
                                                     reproduction, fix,
                                                     regression check

  Research & technical                           15% Sources, synthesis,
  judgment                                           experiments,
                                                     comparisons,
                                                     recommendation

  Testing, reliability                           15% Test strategy, edge
  & security thinking                                cases, resilience,
                                                     threat/risk analysis

  Code comprehension &                           10% Trace, review
  review                                             findings,
                                                     prioritization,
                                                     evidence

  Delivery &                                      5% Git diff/commit,
  engineering workflow                               reproducibility,
                                                     submission;
                                                     CI/deployment only if
                                                     relevant

  Technical                                   **5%** Relevant
  communication /                                    explanations,
  avatar conversation                                clarifying questions,
                                                     evidence-based
                                                     collaboration

  **Total**                                 **100%** 
  ------------------------------------------------------------------------

This broad profile gives **95%** of the illustrative score to technical
work and engineering workflow, and **5%** to the communication/avatar
dimension. If the role is research-heavy, reliability-heavy, or
platform-heavy, adjust the technical dimensions accordingly; do not
automatically increase avatar weight. If a competency is not assessed,
do not silently redistribute its weight as if the candidate had
demonstrated it. Either gather suitable evidence or mark the dimension
as unassessed and explain how the role-level summary was computed.

### Role-specific weighting

A recruiter may configure a role-specific rubric, for example: -
Research engineer: more weight on research quality, experiment design,
and technical synthesis. - Backend/platform engineer: more weight on
design, correctness, reliability, testing, and debugging. - Frontend
engineer: more weight on implementation, interaction behavior,
accessibility, testing, and maintainability. - SRE/reliability engineer:
more weight on debugging, observability, incident response, and fault
tolerance. - Early-stage generalist: balanced evidence across
implementation, design, debugging, testing, and delivery.

The avatar should remain bounded. A reasonable policy is to keep
communication/avatar evidence at **5--10% by default**, and to require
an explicit role-based justification and calibration evidence before
going above that. It should never silently become the deciding factor.

### Score interpretation

-   **Competency score:** a rubric-based judgment for one dimension.
-   **Evidence confidence:** how strong and sufficient the supporting
    evidence is.
-   **Evidence coverage:** how much of the intended competency was
    actually observed.
-   **Role relevance:** whether the competency is essential, useful, or
    not required.
-   **Overall summary:** a transparent synthesis, not a substitute for
    the individual dimensions.

Do not conflate score and confidence. A high score with low confidence
is not the same as a high score with strong evidence.

### Example competency record

``` json
{
  "competency": "Debugging and root-cause reasoning",
  "score": 4,
  "scale": "1-5 rubric",
  "confidence": "high",
  "coverage": "sufficient",
  "evidence": [
    "reproduced the failing test",
    "identified the incorrect state transition",
    "added a regression test"
  ],
  "limitations": [
    "did not assess distributed tracing"
  ],
  "review_required": false
}
```

The final report must link each finding to the actual artifact or event
that supports it.

------------------------------------------------------------------------

### Quantitative researcher assessment blueprint

Mindfries should support quantitative research as a first-class role family. The objective is not to reward a candidate merely for producing an impressive backtest or fluent financial vocabulary. It is to assess whether the candidate can turn a hypothesis into a mathematically defensible, statistically valid, reproducible investigation; identify when evidence is weak; and communicate what can and cannot be concluded.

This blueprint uses the same V2 assessment engine, evidence timeline, local CLI, candidate-preferred IDE, floating tabbed workspace, policy-controlled AI Help Assistant, and human-reviewed capability report. It is a role template, not a separate product. The recruiter selects a track and seniority, and Mindfries selects tasks and follow-ups that cover the essential competencies without treating untested skills as weaknesses.

#### Quant research tracks

- **Systematic strategies / alpha research:** probability and statistics, time series, hypothesis testing, feature construction, portfolio construction, backtesting, transaction costs, risk, and robustness.
- **Derivatives / pricing research:** stochastic processes, options and derivatives pricing, volatility modeling, calibration, numerical methods, sensitivities, and model assumptions.
- **Machine-learning quantitative research:** feature engineering, time-aware validation, leakage prevention, model comparison, calibration, regime sensitivity, and reproducibility.

A candidate can be assessed against one primary track with selected shared-core tasks. Do not assume that every quant role requires the same blend of stochastic calculus, market microstructure, portfolio construction, or ML.

#### Standard assessment flow

1. **Hypothesis and experiment plan (10–15 minutes).** Present a market observation. Ask the candidate to state a falsifiable hypothesis, a null hypothesis, assumptions, target variable, observation horizon, baseline, and criteria that would count against the hypothesis.
2. **Data investigation (15–20 minutes).** Provide a versioned, synthetic or licensed historical dataset containing realistic quality problems. The candidate profiles timestamps, missing observations, outliers, corporate actions, data availability, and potential selection bias before modeling.
3. **Implementation and baseline (25–35 minutes).** The candidate writes code in their own supported IDE, creates a simple baseline, implements the proposed analysis or strategy, and makes the experiment reproducible.
4. **Validation and risk (15–20 minutes).** Require chronological splits or walk-forward validation where appropriate, transaction-cost assumptions, robustness checks, and a comparison with a sensible baseline. The evaluator should distinguish statistical evidence from a trading recommendation.
5. **Unexpected evidence / research update (10–15 minutes).** Introduce one realistic complication from the scenario library below. Evaluate whether the candidate revises the hypothesis and analysis safely rather than defending the original result at all costs.
6. **Research handover (5–10 minutes).** Ask for a concise conclusion, limitations, reproducibility instructions, unresolved questions, and the next experiment. A well-supported conclusion that the signal is not robust can be a strong outcome.

Durations are configurable. A shorter screen should use a smaller, well-defined subset rather than pretending to assess the entire discipline in one sitting.

#### Core worked example: momentum signal

**Candidate prompt:** “You observe that an asset’s short-term momentum appears to predict its next-period returns. Investigate whether the relationship is statistically meaningful and whether a simple strategy based on it survives realistic validation.”

Expected work products:

- Dataset inspection and documented cleaning decisions.
- A clear hypothesis and baseline, including why the chosen statistical test is appropriate.
- Runnable analysis or strategy code with fixed configuration and reproducible outputs.
- A time-ordered train/validation/test design or walk-forward procedure; no random shuffling of future and past observations when that would leak information.
- Explicit execution timing, trading costs, turnover, and benchmark assumptions.
- Relevant performance and risk measures, including drawdown and uncertainty; avoid treating a single metric such as Sharpe ratio as proof.
- Robustness checks across periods, parameter choices, or reasonable alternative assumptions.
- A short report stating what the evidence supports, what it does not support, and what should be tested next.

There is no guaranteed profitable answer. The assessment must not reward a high in-sample return more than sound methodology. Candidates should be allowed to reject the initial premise if their investigation supports that conclusion.

#### Additional unusual and adversarial scenario library

Use one or two scenarios per assessment, selected to match the role and level. They should be strange enough to expose brittle reasoning, but grounded in plausible research failures. They must not be arbitrary riddles or depend on guessing what the evaluator is thinking.

**Scenario A — The future that arrived early (look-ahead leakage).** A feature called `next_day_close` is present in the dataset because an upstream join shifted a column by one row. The initial backtest looks extraordinary. Strong candidates inspect feature timestamps and data lineage, detect that information would not have been available at decision time, repair the join, rerun the experiment, and disclose the impact. Do not score a candidate down merely for not spotting a hidden column name; provide enough evidence to investigate timestamps and joins.

**Scenario B — The vanished companies.** The dataset contains only assets that still exist today. A strategy appears profitable. Ask whether the sample represents the investable universe at each historical date. Strong responses identify survivorship/selection bias and explain how delisted assets and historical membership would change the analysis. A candidate should be able to describe the limitation even if the assessment does not include a complete point-in-time universe.

**Scenario C — The split that looks like a crash.** One instrument has a large apparent overnight price collapse around a stock split or other corporate action. A naive return calculation treats it as a catastrophic loss. Look for investigation of adjustment conventions, corporate-action metadata, and consistent treatment across price and volume rather than a hard-coded deletion of the outlier.

**Scenario D — The clock is lying.** Price bars are labeled in mixed time zones, a daylight-saving transition creates duplicate or missing timestamps, or a market-close event is assigned to the next session. This can create false lead-lag relationships. Strong candidates normalize timestamps carefully, establish market-session semantics, and test whether results survive a corrected alignment.

**Scenario E — The perfect Sharpe ratio.** A backtest reports an implausibly smooth equity curve with almost no drawdowns. The cause could be stale prices, forward-filled returns, overlapping labels, a wrong return denominator, or a bug in portfolio aggregation. The task is to investigate the measurement pipeline before celebrating the result. Do not require one predetermined root cause; score the candidate’s diagnostic process and verification.

**Scenario F — The strategy only works after the press release.** A signal is calculated using an earnings value but trades at a price that predates the value’s public release. Ask the candidate to build an availability-time rule and rerun the test. Distinguish event time, publication time, ingestion time, and trade time where relevant.

**Scenario G — The transaction-cost monster.** A strategy has positive gross returns but high turnover. Reveal a realistic spread, fee, slippage, or market-impact estimate after the first result. Evaluate whether the candidate recalculates net performance, investigates capacity, and communicates sensitivity rather than treating costs as an afterthought.

**Scenario H — The one-week wonder.** Nearly all profits occur during one unusual market week or one volatile regime. Ask for period-by-period diagnostics, regime sensitivity, and a decision about whether the effect is robust or merely a historical coincidence. The candidate should not be forced to invent a regime explanation without evidence.

**Scenario I — The multiple-testing trap.** The candidate tried hundreds of parameter combinations and reports only the best one. Ask how the research process should account for selection effects, multiple comparisons, and an untouched holdout. Reward transparent experiment logs and preregistered or otherwise disciplined evaluation, not just a post-hoc significance test.

**Scenario J — The missing data is the signal.** Missing prices cluster around volatile periods or illiquid assets. A simplistic forward-fill makes the strategy look stable. Strong candidates investigate whether missingness is systematic, avoid silently fabricating tradable prices, and report sensitivity to defensible treatments.

**Scenario K — The synthetic-data decoy.** The dataset is generated from a known process with no intended predictive signal, but finite-sample noise makes one feature look useful. The candidate should check baselines, uncertainty, and out-of-sample behavior and may reasonably conclude that there is insufficient evidence. Do not reveal the generator’s answer until after submission, and do not grade the candidate on guessing it.

**Scenario L — The broken benchmark.** The strategy is compared against a benchmark with a different date range, return frequency, currency, or dividend treatment. Ask the candidate to audit comparability before interpreting excess performance.

**Scenario M — The impossible fill.** The backtest enters at the same bar’s closing price even though the signal uses that close, or assumes unlimited fills at a quoted mid-price. Look for an explicit decision-time/execution-time model and a conservative fill assumption.

**Scenario N — The feature that knows the ticker.** An ML model performs well because the asset identifier or an encoding of the target universe leaks future membership or labels. Ask the candidate to inspect feature provenance, split strategy, and generalization to unseen assets or periods when that is relevant to the role.

**Scenario O — The regime changed after deployment.** A once-useful relationship weakens after a structural market change. Ask the candidate to design monitoring, drift checks, retraining or retirement criteria, and a cautious research response. Do not reward an automatic retraining proposal without considering leakage, costs, and validation.

**Scenario P — The tiny illiquid asset.** A strategy’s best returns come from instruments that could not plausibly support the assumed position size. Require the candidate to consider liquidity, capacity, position limits, and whether results survive realistic constraints.

**Scenario Q — The unit mismatch.** One feed reports percentages, another decimals; a volatility column is annualized while the model expects daily values; or prices are in different currencies. The candidate should verify units and conventions, trace the issue to source data, and rerun affected outputs. This tests disciplined debugging, not memorization of a particular market convention.

**Scenario R — The model is right for the wrong reason.** A predictive model relies heavily on a feature that is a proxy for a data vendor’s publication schedule or a market-close artifact. Ask for feature ablation, stability checks, and a causal caution: predictive association alone does not establish an economic mechanism.

**Scenario S — The rare event that dominates everything.** A risk model looks accurate on ordinary days but fails during a small number of extreme moves. Ask the candidate to discuss tail risk, stress testing, sample limitations, and why a single average loss metric may hide important failure modes.

**Scenario T — The answer changes when the seed changes.** An ML experiment produces materially different conclusions across random seeds or sample windows. Strong candidates report variance, set seeds for reproducibility without pretending that a fixed seed proves robustness, and test stability across reasonable choices.

For every scenario, the evaluator must use a rubric that accepts multiple valid investigative paths. A scenario should provide enough observable evidence to support a diagnosis, and the candidate must have a fair opportunity to inspect it. Avoid hidden tricks whose only purpose is to catch a candidate out.

#### Quant Researcher scoring rubric

Use this as an initial template for a systematic-strategy assessment; calibrate weights with domain experts and hiring outcomes:

- Mathematical and statistical reasoning — **20%**.
- Experimental design and hypothesis testing — **15%**.
- Data quality, point-in-time correctness, and leakage prevention — **15%**.
- Research implementation and reproducibility — **15%**.
- Backtesting and financial modeling — **15%**.
- Risk analysis and robustness — **10%**.
- Interpretation, limitations, and research communication — **10%**.

For derivatives/pricing roles, shift weight toward stochastic modeling, numerical methods, calibration, and sensitivity analysis. For ML-driven quant roles, shift weight toward feature validation, model comparison, temporal generalization, and leakage control. Communication/avatar evidence remains bounded and cannot replace research artifacts.

Evaluate the process as well as the result: whether the candidate investigates before changing code; states assumptions; chooses appropriate baselines; preserves temporal correctness; validates outputs; tracks experiments; distinguishes uncertainty from certainty; and updates conclusions when new evidence contradicts the original hypothesis. A candidate must not be rewarded for cherry-picking, excessive complexity, or unsupported claims of profitability.

#### Quant workspace tabs and evidence

The shared floating panel can expose the following tabs based on the recruiter’s assessment blueprint:

- **Research Lab:** hypothesis, null hypothesis, experiment plan, assumptions, and conclusion.
- **Data Explorer:** schema, distributions, missingness, timestamp checks, and data-quality notes.
- **Model / System Design:** feature pipeline, modeling assumptions, validation plan, and architecture where relevant.
- **Debugging:** inspect failures, suspicious metrics, data joins, and calculation discrepancies.
- **AI Help:** policy-controlled assistance for reasoning and debugging; tool actions remain governed by the assessment policy.
- **Notes & Evidence:** experiment log, result snapshots, references, decisions, and handover.
- **Local IDE / terminal:** the candidate’s own supported coding environment, connected to the assessment through the CLI and scoped local agent.

Evidence should include the hypothesis and its revisions, code and diffs, dataset/version identifiers, configuration, experiment outputs, validation checks, relevant tool events, and the final report. Private notes must not become submitted evidence silently. Use synthetic or appropriately licensed data; never ask candidates to upload private brokerage credentials, personal trading accounts, or confidential employer datasets.

#### Quant assessment report

The recruiter-facing report should show:

- Per-competency score, evidence confidence, and evidence coverage.
- Hypothesis quality and whether the experiment could falsify it.
- Data integrity and point-in-time correctness checks.
- Reproducibility of the implementation and experiment outputs.
- In-sample versus out-of-sample findings and sensitivity to assumptions.
- Risk, costs, capacity, and limitations relevant to the selected track.
- What the candidate concluded, what remains unknown, and suggested follow-up questions.

Do not produce a simplistic “profitable / not profitable” hiring verdict. Historical backtest performance is not proof of future profitability, and a negative result can demonstrate excellent research judgment. Reports should support human review and make the evidence and caveats inspectable.

---

## 15. Evidence model and timeline

Every important evaluation should be traceable to an evidence item.

### Evidence types

-   Code artifact: file, diff, commit, test, build output.
-   Design artifact: diagram, decision record, alternatives,
    assumptions.
-   Research artifact: source, note, hypothesis, experiment, synthesis.
-   Debugging artifact: reproduction, log/trace reference, hypothesis,
    fix, verification.
-   Testing artifact: test plan, test cases, failure analysis.
-   Delivery artifact: Git status, commit, submission manifest, optional
    CI/deployment result.
-   Conversation artifact: question, answer, linked work item, rubric
    criterion.
-   Environment artifact: setup result, dependency error, network
    interruption, recovery.
-   Integrity event: observed action or technical signal, policy
    mapping, verification status, review outcome.

### Standard evidence record

Each record should include: - Unique ID and session ID. - Timestamp and
event source. - Artifact or event type. - Relevant competency/rubric
criterion. - A reference to the artifact, not only a free-form AI
summary. - What was directly observed. - Any interpretation, clearly
separated from observation. - Confidence and known limitations. -
Policy/consent basis for collection. - Evaluator/model version where
applicable. - Whether human review is required.

### Evidence timeline example

``` text
10:02 — Workspace preflight passed
10:07 — Candidate inspected existing service and documented assumptions
10:15 — Research note added with two approaches and source rationale
10:23 — First implementation committed locally
10:29 — Test failed; candidate reproduced and narrowed the failure
10:36 — Candidate updated design canvas to reflect a discovered constraint
10:42 — Regression test passed
10:47 — Avatar asked about an explicit trade-off; candidate linked answer to design note
10:55 — Submission manifest verified
```

The timeline should be factual and neutral. It should not label a
candidate "suspicious" merely for switching windows, typing slowly,
using search, or taking a break.

------------------------------------------------------------------------

## 16. Integrity monitoring and anti-cheating design

### Use three distinct categories

1.  **Observed event:** a factual event the system can actually verify,
    such as a permitted process reporting a Git commit or an assessment
    window losing focus where that signal is available.
2.  **Verified policy violation:** a concrete event that clearly
    conflicts with a disclosed rule and has been verified with adequate
    evidence.
3.  **Review signal:** an ambiguous or incomplete signal that may
    deserve human review but does not establish wrongdoing.

Never merge these into a single opaque "cheating score."

### Possible assessment-relevant signals (only when disclosed and technically available)

-   Assessment agent start/stop and heartbeat.
-   Session connection/disconnection.
-   Workspace creation and repository state.
-   Git status, commits, diffs, and submission events.
-   Commands run in the assessment workspace, when allowed and relevant.
-   Build/test results and relevant process exit codes.
-   Focus/visibility changes in the Mindfries browser session, if used
    and disclosed.
-   Changes to assessment files or task versions.
-   Use of an AI tool only where that use can be reliably observed or
    voluntarily declared under the assessment policy.
-   Repeated submission or credential/session anomalies.

### Signals that are not proof by themselves

-   Browser tab or window switching.
-   A long pause or a burst of typing.
-   Copy/paste without context.
-   A correct answer that resembles an online answer.
-   A candidate's accent, facial expression, eye movement, or
    confidence.
-   A candidate challenging the avatar or disagreeing with an evaluator.
-   Use of search, documentation, or an approved AI tool.
-   A local agent disconnecting or a machine crashing.

### Integrity workflow

``` text
Relevant signal observed
        |
        v
Record factual event + policy context
        |
        v
Can the system verify a concrete rule violation?
        |                         |
       Yes                        No
        |                         |
        v                         v
Mark “verified violation”     Mark “review signal”
with supporting evidence      and record uncertainty
        |                         |
        +------------+------------+
                     |
                     v
          Human review if material
                     |
                     v
    Candidate can provide context / appeal
                     |
                     v
     Record final review outcome and reason
```

### Privacy and security safeguards

-   Show a plain-language monitoring notice before the session.
-   Collect only what is relevant to the stated assessment purpose.
-   Do not collect passwords, private messages, unrelated personal
    files, or broad keystroke logs.
-   Avoid screenshots, webcam, microphone, or screen recording unless
    specifically necessary, separately disclosed, and consented to.
-   Make voice recording optional where feasible; offer text
    alternatives.
-   Encrypt evidence in transit and at rest.
-   Restrict access by role; log reviewer access to sensitive evidence.
-   Set a retention period and automatic deletion policy.
-   Provide a process for candidates to challenge an integrity finding.
-   Do not use monitoring events as a substitute for evaluating
    technical capability.
-   If stronger lockdown is needed, offer a separately described managed
    environment; do not claim a standard CLI provides it.

------------------------------------------------------------------------

## 17. Git, GitHub, CI, and deployment

### Git workflow

1.  CLI prepares the assessment repository or starter workspace.
2.  Candidate works on an assessment branch.
3.  Mindfries periodically verifies local workspace status where
    permitted.
4.  Candidate reviews the final diff and can write a commit message.
5.  Candidate explicitly submits.
6.  CLI creates a submission manifest containing the task version,
    commit/hash where available, changed files, test/build results, and
    timestamp.
7.  Backend verifies that the received artifact matches the submission
    manifest.

### GitHub options

-   **Option A --- Platform submission:** no GitHub account required;
    upload the relevant patch or workspace artifact through the
    authorized channel.
-   **Option B --- Private temporary repository:** candidate authorizes
    GitHub access and Mindfries creates or uses a private assessment
    repository.
-   **Option C --- Candidate-owned repository:** candidate explicitly
    chooses and authorizes a repository when appropriate.

Never push publicly by default. Explain the destination, visibility, and
permissions before the candidate authorizes it. Avoid storing broad or
long-lived GitHub tokens. Revoke or expire credentials when the session
ends.

### CI and deployment

CI/deployment should be enabled only when it is meaningful to the
role: - Use a standardized, isolated environment. - Never require
candidates to expose real secrets or use personal cloud billing. - Use
mock credentials and least-privilege tokens. - Make deployment failure
distinguishable from code failure, account setup failure, or platform
outage. - If deployment is assessed, define expected evidence:
successful build, health check, logs, rollback plan, or reproducible
instructions. - Do not make public deployment mandatory for unrelated
roles.

------------------------------------------------------------------------

## 18. Evaluation engine

### Evaluation stages

1.  **Artifact validation:** confirm the task version, file manifest,
    commit/diff, and submission state.
2.  **Deterministic checks:** run tests, linting, build checks, schema
    validation, or reproducibility checks where relevant.
3.  **Artifact-specific evaluation:** evaluate code, design, research,
    debugging, tests, and delivery against their own rubrics.
4.  **Cross-artifact consistency:** check whether design assumptions
    match implementation, tests support claimed behavior, and research
    recommendations follow from evidence.
5.  **Conversation evidence:** attach relevant avatar answers to the
    specific work and criterion they clarify.
6.  **Confidence and coverage:** determine whether the evidence is
    sufficient and reliable.
7.  **Targeted follow-up:** ask only when an important criterion remains
    uncertain and the assessment allows it.
8.  **Human review:** review material ambiguities, integrity flags, and
    borderline decisions.
9.  **Report generation:** produce a traceable capability profile.

### Model and evaluator safeguards

-   Use structured outputs with explicit rubric criteria.
-   Require evidence references for qualitative claims.
-   Keep evaluator/model versions and prompts auditable.
-   Do not let a single model grade every artifact without cross-checks.
-   Use deterministic checks where they are appropriate.
-   Calibrate rubrics against expert-reviewed sample submissions.
-   Measure evaluator agreement and false-positive/false-negative rates.
-   Re-evaluate a sample of submissions with human reviewers.
-   Test for systematic scoring differences across relevant candidate
    groups.
-   Do not generate a precise numeric score when evidence is
    insufficient.

------------------------------------------------------------------------

## 19. Candidate report / capability profile

The result should be a profile, not just "Candidate: 82/100."

### Recommended report sections

1.  **Role-relevant summary:** concise synthesis of demonstrated
    strengths and limitations.
2.  **Competency matrix:** score, confidence, evidence coverage, and
    essential/differentiator status for each dimension.
3.  **Evidence links:** code diffs, tests, design artifacts, research
    notes, debugging trail, and delivery artifacts.
4.  **Strongest demonstrated capabilities:** grounded in specific
    evidence.
5.  **Areas needing follow-up:** only where relevant to the role and
    supported by evidence.
6.  **Not assessed / insufficient evidence:** explicit unknowns.
7.  **Environment and accommodation notes:** technical interruptions or
    alternate routes that affect interpretation.
8.  **Integrity review:** neutral events, verified violations if any,
    unresolved signals, and human-review outcome.
9.  **Suggested interview follow-up:** focused questions derived from
    evidence gaps---not generic interview questions.
10. **Human decision and rationale:** recruiter decision, reviewer
    notes, and audit history.

### Example capability matrix

  ---------------------------------------------------------------------------------
  Competency            Result         Confidence     Coverage       Evidence
  --------------------- -------------- -------------- -------------- --------------
  Implementation        Strong         High           Sufficient     Diff, tests,
                                                                     run result

  System design         Strong         Medium         Sufficient     Canvas,
                                                                     trade-off
                                                                     notes

  Research              Strong         High           Sufficient     Source
                                                                     comparison,
                                                                     experiment

  Debugging             Developing     High           Sufficient     Root-cause
                                                                     trail,
                                                                     regression
                                                                     test

  Testing/reliability   Strong         High           Sufficient     Test plan,
                                                                     edge cases

  Code review           Not assessed   ---            None           No review task
                                                                     assigned

  Delivery              Meets          High           Sufficient     Commit and
                        expectations                                 manifest

  Technical             Meets          Medium         Limited        Two contextual
  communication         expectations                                 explanations
  ---------------------------------------------------------------------------------

------------------------------------------------------------------------

## 20. Accessibility, fairness, and candidate experience

-   Offer text and voice interaction where feasible.
-   Support keyboard navigation and screen readers in browser
    workspaces.
-   Avoid relying on speed alone unless speed is an explicit, validated
    job requirement.
-   Provide clear instructions, examples of deliverable formats, and
    permitted-tool rules.
-   Support reasonable time accommodations.
-   Do not evaluate accent, facial expression, eye contact, or perceived
    personality.
-   Ensure avatar questions are understandable, relevant, and not
    needlessly adversarial.
-   Provide a way to ask for clarification without penalizing the
    candidate.
-   Allow the candidate to report a technical issue during the
    assessment.
-   Keep assessment variants comparable and review their outcomes for
    unintended bias.
-   Make it possible to challenge incorrect evidence or a material
    integrity finding.

------------------------------------------------------------------------

## 21. MVP-to-V2 implementation plan

### Phase 1 --- Assessment foundation

-   Role blueprint and versioned rubrics.
-   Candidate session and consent flow.
-   Task delivery and a common core.
-   Code workspace preparation.
-   Git status/diff and reliable submission.
-   Evidence schema and report with explicit unknowns.
-   Human review workflow.

### Phase 2 --- Local CLI and reliability

-   Signed/versioned CLI.
-   Environment preflight.
-   Git configuration and preferred IDE launch.
-   Short-lived session agent.
-   Local event queue and reconnect behavior.
-   Session cleanup and scoped credentials.
-   Environment-failure handling.

### Phase 3 --- Multi-mode assessment

-   System Design Canvas.
-   Research Lab.
-   Debugging/reliability tasks.
-   Code comprehension/review.
-   Testing and delivery evidence.
-   Cross-artifact linking.

### Phase 4 --- AI avatar and adaptive follow-ups

-   Contextual questions tied to candidate artifacts.
-   Voice/text controls and pause/defer.
-   Conversation evidence linked to rubrics.
-   Adaptive route selection and follow-up limits.
-   Avatar contribution bounded in the rubric.
-   Evaluation calibration and human-review triggers.

### Phase 5 --- Integrity, integrations, and calibration

-   Disclosed event timeline.
-   Separate observed-event, verified-violation, and review-signal
    categories.
-   GitHub private repository authorization.
-   Optional CI/isolated deployment.
-   Retention controls, reviewer audit logs, candidate response process.
-   Fairness checks and evaluator agreement studies.

### Phase 6 --- Production hardening

-   Load and failure testing.
-   Threat modeling and security review.
-   Dependency and artifact scanning.
-   Monitoring for service reliability, latency, cost, and evaluation
    drift.
-   Support operations and incident response.
-   Controlled rollout with candidate and recruiter feedback.

------------------------------------------------------------------------

## 22. Success metrics

### Assessment quality

-   Agreement between independent expert reviewers.
-   Evidence coverage per essential competency.
-   Percentage of qualitative findings with valid evidence links.
-   Calibration error and evaluator disagreement rate.
-   Rate of "not assessed" outcomes for essential competencies.

### Candidate experience

-   Setup completion rate.
-   Time to first successful workspace launch.
-   Environment failure rate and recovery time.
-   Assessment completion and abandonment rate.
-   Candidate-reported clarity, fairness, and accessibility.
-   Avatar interruption and mute/pause rates as product feedback---not
    as candidate scores.

### Reliability

-   Event sync success and duplicate rate.
-   Submission integrity verification success.
-   Agent crash/recovery rate.
-   Session recovery success after disconnect.
-   API availability and report-generation latency.

### Hiring utility

-   Reviewer time per assessment.
-   Agreement between evidence-based assessment and structured
    downstream job performance measures, where available and lawful.
-   Usefulness of evidence links and follow-up questions.
-   Recruiter ability to explain why a candidate was progressed or not.

### Integrity quality

-   Human-confirmed precision of integrity flags.
-   Candidate appeal rate and overturned findings.
-   Rate of technical interruptions incorrectly routed to integrity
    review.
-   Completeness of consent and retention-policy records.

Do not optimize for the number of people flagged or rejected. Optimize
for valid evidence, fair decisions, and reliable assessment.

------------------------------------------------------------------------

## 23. Key product decisions

1.  **No IDE extension in V2:** use a candidate-invoked CLI and
    short-lived local agent.
2.  **Browser + local IDE:** the browser coordinates the assessment; the
    candidate uses a supported IDE of their choice.
3.  **Multiple workspaces:** code, design, research, debugging, review,
    testing, and delivery share one evidence model.
4.  **Adaptive, not arbitrary:** a common core plus targeted routes,
    with stable competency rubrics.
5.  **Avatar is secondary:** default communication/avatar contribution
    is 5% in the illustrative broad-engineering profile; technical
    evidence carries the overwhelming majority.
6.  **No single-score hiring:** provide per-competency scores, evidence,
    confidence, coverage, and unknowns.
7.  **Integrity is evidence-based:** observed events, verified
    violations, and review signals are separate.
8.  **No claim of total surveillance:** the standard CLI cannot observe
    every app or prove all external AI use.
9.  **GitHub/deployment are optional by role:** private/platform
    submission remains available.
10. **Human decision stays human:** Mindfries supports hiring decisions;
    it does not replace accountable review.

------------------------------------------------------------------------

## V2 release gate: do not cut over before all checks pass

This checklist is mandatory. A feature being implemented is not equivalent to V2 being ready.

### Product and assessment quality
- [ ] Candidate can complete the entire V2 flow from onboarding through final submission.
- [ ] The selected assessment routes evaluate the role’s essential competencies.
- [ ] Research, design, implementation, debugging, testing, review, and delivery evidence is linked to rubric criteria.
- [ ] The AI Help Assistant follows the configured policy, and blocked tool actions cannot be executed through the backend.
- [ ] Approved developer services such as Supabase work only with the configured, scoped permissions.
- [ ] Avatar/conversation evidence remains low-weight and cannot override strong artifact evidence.
- [ ] “Not assessed” and “insufficient evidence” are represented distinctly from low performance.
- [ ] Reports expose evidence, confidence, coverage, caveats, and human-review outcomes.

### Local tooling and delivery
- [ ] CLI download and integrity verification are tested on each supported OS.
- [ ] Session credentials are short-lived, scoped, and cleaned up.
- [ ] Preflight failures, IDE launch failures, dependency failures, network loss, and agent crashes have recovery paths.
- [ ] Git diffs and submission manifests are verified.
- [ ] GitHub access is private, scoped, authorized, and revocable; platform submission works without GitHub.
- [ ] CI/deployment is isolated and optional unless required by the role.

### Privacy, integrity, and security
- [ ] Candidate receives a clear notice of what is and is not collected.
- [ ] The implementation does not claim that a normal CLI can see or block every external application.
- [ ] Observed events, verified violations, and review signals are separate categories.
- [ ] Sensitive evidence access is permissioned and audited.
- [ ] Retention/deletion and candidate-response processes are tested.
- [ ] Threat modeling and security review are complete.

### V1 non-regression and coexistence
- [ ] V1 candidate portal works as before.
- [ ] V1 browser-based IDE works as before.
- [ ] V1 dashboard works as before.
- [ ] V1 AI chatbot works as before.
- [ ] V1 authentication, session lifecycle, data persistence, integrations, and background jobs remain healthy.
- [ ] V1 regression suite passes against the V1 deployment.
- [ ] V2 deploys and fails independently without corrupting V1.
- [ ] V2 migrations, jobs, credentials, and test data are isolated from V1.
- [ ] Rollback has been rehearsed and does not require restoring V1 data from backup.
- [ ] Explicit go/no-go approval has been recorded.

**Release rule:** if any critical V1 non-regression, data-isolation, security, or submission-integrity check fails, V2 does not cut over. Continue testing V2 on its isolated path while V1 remains the live product.

## 24. Final product vision

Mindfries V2 should feel less like an exam and more like a well-designed
engineering work session. A candidate receives a meaningful problem,
chooses or follows a fair route, works in their own environment,
researches and designs where appropriate, builds or investigates,
explains key decisions, and submits verifiable work. The platform
connects those artifacts into a coherent record of demonstrated
capability.

The hiring team does not just see a number. They see what the candidate
did, how they reasoned, what the evidence supports, where confidence is
limited, and what still needs to be assessed. A strong researcher is not
hidden by a coding-only test; a strong debugger is not hidden by a
generic interview; and a quiet candidate is not penalized for speaking
less when their work already provides strong evidence.

**Mindfries V2 evaluates engineering through demonstrated work first,
contextual conversation second, and human judgment throughout.**
