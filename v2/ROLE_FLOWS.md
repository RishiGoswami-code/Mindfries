# Role-specific candidate assessment flows: FDE and DevOps

Two worked examples of how an assessment runs inside Mindfries V2, end to
end. They show how the pieces in [`SPEC.md`](SPEC.md) fit together for a
real role; where they and the spec disagree, the spec wins.

> **Note:** these flows predate the decision to move the assistant into the
> terminal. Where they say "AI Help", that is now `mindfries agent`, run from
> the CLI. The customer conversation and the other tabs stay in the floating
> panel. See [README](README.md#where-this-repos-v2-departs-from-the-spec).

These are end-to-end examples of how an assessment should run inside Mindfries V2. They are not standalone interview scripts. Both use the shared recruiter blueprint, candidate onboarding, local CLI, floating tabbed workspace, evidence timeline, evaluation engine, and reviewer report. Only the scenario, enabled tabs, task progression, and role rubric change.

- [Flow A — Forward Deployed Engineer (FDE)](#flow-a--forward-deployed-engineer-fde-solve-a-customer-problem-in-an-unfamiliar-environment)
- [Flow B — DevOps Engineer](#flow-b--devops-engineer-diagnose-repair-and-safely-release-a-failing-deployment)
- [Shared Mindfries platform behavior across both flows](#shared-mindfries-platform-behavior-across-both-flows)

## Flow A — Forward Deployed Engineer (FDE): solve a customer problem in an unfamiliar environment

### Recruiter setup

1. The recruiter chooses Forward Deployed Engineer, seniority, time budget, and the competencies to assess: customer discovery, integration, implementation, production readiness, delivery, and customer ownership.
2. Mindfries loads a scenario template such as a support team that wants to triage tickets and suggest responses. The recruiter can configure the customer context, sample data, API behavior, privacy constraints, success metrics, and one or two scenario variants.
3. The recruiter chooses an AI-use policy, approved developer tools, the required deliverables, and which optional integrations (for example, a sandbox database, GitHub, or deployment) are relevant.
4. Mindfries previews the customer brief, simulated-customer responses, task files, enabled tabs, evidence collection, rubric, and candidate report. The recruiter or a second reviewer dry-runs the scenario before publication.

### Candidate journey inside Mindfries

| Stage | Candidate experience in the platform | Workspace tabs and system behavior | Evidence captured |
|---|---|---|---|
| 1. Start and understand (3–5 min) | Opens the assessment link, reviews the task, timing, AI policy, privacy notice, and deliverables; completes consent and environment checks. | Mindfries shows the task overview and allowed tools. The CLI is offered only if local code work is needed. | Acknowledgements, preflight status, task version, accessibility or environment issues. |
| 2. Discover the customer problem (10–15 min) | Talks to a simulated customer and asks clarifying questions before committing to a solution. Some constraints are revealed in response to relevant questions. | AI Help / Customer Conversation supports the configured customer interaction; Notes & Evidence lets the candidate record requirements and open questions. The assistant must not simply hand over the solution. | Questions asked, requirements, assumptions, success metrics, constraints, and unresolved unknowns. |
| 3. Investigate the environment (10–15 min) | Examines a starter repository, sample data, API documentation, and integration behavior. | Research Lab and Debugging are available alongside the candidate's Local IDE / terminal. Documentation and tool access follow the task's allowlist. | Data-quality findings, API observations, investigation notes, hypotheses, and reproduction steps. |
| 4. Design a minimum viable solution (10–15 min) | Chooses a practical first slice and explains architecture, data flow, permissions, failure handling, and trade-offs. | System Design Editor and Notes & Evidence capture the design. Mindfries can ask a role-relevant follow-up if a key risk or assumption has not been addressed. | Architecture, alternatives considered, security boundaries, delivery plan, and explicit trade-offs. |
| 5. Build and verify (30–45 min) | Implements a vertical slice in their preferred local IDE, integrates the relevant service, and tests it against sample inputs. | The Local IDE / terminal remains the primary coding environment. The floating panel stays synchronized. The CLI reports scoped, permitted session evidence; it does not take unrestricted control of the computer. | Code diff, tests, run instructions, integration results, sample outputs, and verification steps. |
| 6. Respond to a changed condition (10–15 min) | Receives a realistic complication such as an API rate limit, malformed record, expired sandbox credential, privacy constraint, or customer request for an unsafe shortcut. The candidate diagnoses the issue and adapts the plan. | Debugging, AI Help, and Notes & Evidence remain available under the same visible policy. A scenario can branch based on prior work, but should not punish a candidate for choosing a different valid approach. | Diagnosis, change made, validation, customer communication, risk decision, and any revised assumptions. |
| 7. Demo and hand over (5–10 min) | Demonstrates what works, explains limitations, describes how success would be measured, and proposes next steps. | Mindfries prompts for a concise handover and lets the candidate review the submission manifest before submitting. | Working deliverable, known limitations, outcome measures, runbook or handover notes, and final explanation. |

### What Mindfries evaluates

| Competency | Starting weight |
|---|---|
| Implementation and integration | 20% |
| Customer discovery | 15% |
| Debugging and data investigation | 15% |
| System design and trade-offs | 15% |
| Production readiness, reliability, and security | 15% |
| Delivery and validation | 10% |
| Customer collaboration | 5% |
| Product judgment and reusable improvements | 5% |

These are starting weights and must be calibrated for seniority and specialization.

### Example scenario variations

The customer asks for automation before defining success; the API documentation is slightly stale; duplicate tickets make the first demo look better than it is; a data field contains sensitive information; rate limits appear only at higher volume; or a stakeholder changes the priority halfway through. These should test prioritization and evidence-based judgment, not reward guessing a hidden answer.

### Final FDE report

Show whether the candidate clarified the problem, what they built, whether it worked on the provided cases, how they validated it, what risks remain, and what evidence supports each competency. Separate score, evidence confidence, and evidence coverage. Mark competencies as not assessed when the task did not give the candidate a fair opportunity to demonstrate them.

## Flow B — DevOps Engineer: diagnose, repair, and safely release a failing deployment

### Recruiter setup

1. The recruiter chooses DevOps Engineer and selects a focus: general DevOps, Kubernetes/platform engineering, CI/CD, infrastructure as code, or SRE/incident response.
2. Mindfries prepares an isolated, disposable environment with a starter application, Dockerfile or equivalent packaging, deployment configuration, pipeline, sanitized logs, health-check behavior, and an expected-state description. The environment includes a primary failure and may include a second realistic issue.
3. The recruiter configures allowed commands and tools, AI-use policy, whether Terraform/Kubernetes/cloud simulation is relevant, the rollback requirement, and the evidence expected from the candidate.
4. The scenario is validated so that failures are reproducible, fixes can be verified, and the candidate cannot affect real production systems. Personal cloud credentials are never required.

### Candidate journey inside Mindfries

| Stage | Candidate experience in the platform | Workspace tabs and system behavior | Evidence captured |
|---|---|---|---|
| 1. Start and preflight (3–5 min) | Reads the incident brief, permitted tools, environment boundaries, time rules, and expected deliverables. Runs the provided setup check if local work is required. | Mindfries displays the sandbox boundary and task policy. The CLI checks supported tooling and confirms the target workspace before making changes. | Task acknowledgement, environment readiness, tool versions, and setup errors. |
| 2. Inspect before changing (10 min) | Reviews the repository, deployment pipeline, service configuration, logs, health checks, and dependency map. Forms initial hypotheses. | Debugging, System Design, and Local IDE / terminal are enabled. The workspace makes logs and config available without granting access to a real production environment. | Initial observations, likely failure boundaries, commands or checks run, and ranked hypotheses. |
| 3. Diagnose the failed release (15 min) | Investigates a failed rollout. Possible symptoms include a successful image build followed by a failed readiness check, a wrong environment variable, an image/config mismatch, or a networking issue. | Debugging supports hypothesis tracking; Research allows approved documentation lookup; AI Help follows the assessment's visible policy. Mindfries records relevant assessment events, not indiscriminate personal-device activity. | Root-cause explanation, supporting log/config evidence, reproduction, and distinction between symptom and cause. |
| 4. Make a minimal safe fix (10–15 min) | Changes the relevant configuration or code, adds or repairs health verification, and avoids broad permission changes or hard-coded secrets. | The candidate edits through the Local IDE / terminal. The platform can run deterministic checks and tests in the sandbox. | Diff, tests, configuration changes, security checks, and rationale for the chosen fix. |
| 5. Release or roll back (10–15 min) | Attempts a verified rollout or chooses a safe rollback when release conditions are not met. Demonstrates how success is determined rather than assuming that a command exit code means the service is healthy. | Debugging and Notes & Evidence capture deployment status. Where supported, the sandbox simulates rollout, health checks, and rollback; no real production deployment is required. | Build/test result, rollout state, health verification, rollback decision, and post-change checks. |
| 6. Handle an incident complication (10–15 min) | Receives an additional symptom such as elevated latency, resource exhaustion, a failing dependency, a secret exposed in a test log, or a rising error rate. Prioritizes mitigation and gathers evidence. | System Design, Debugging, Research, and Notes & Evidence are available according to the blueprint. Follow-up difficulty may adapt to what the candidate has already demonstrated. | Triage order, impact assessment, evidence used, mitigation, escalation decision, and prevention plan. |
| 7. Handover and submit (5–10 min) | Writes a concise incident summary and runbook: cause, impact, fix, verification, rollback, monitoring, and follow-up actions. Reviews and submits the deliverables. | Mindfries presents the submission manifest and checks required artifacts. The candidate confirms the final submission. | Reproducible fix, pipeline/config changes, verification output, runbook, known limitations, and final handover. |

### What Mindfries evaluates

| Competency | Starting weight |
|---|---|
| Troubleshooting and root-cause analysis | 20% |
| CI/CD and release engineering | 20% |
| Infrastructure, Linux, and networking | 15% |
| Containers and orchestration | 15% |
| Observability and incident response | 10% |
| Security and secret management | 10% |
| Reliability, rollback, and recovery | 5% |
| Documentation and operational handover | 5% |

Adjust the weights for Kubernetes-heavy, platform engineering, infrastructure-as-code, or SRE roles.

### Example scenario variations

The service is healthy but the readiness probe targets the wrong path; a container works locally but fails in the sandbox because a required environment variable is missing; DNS resolution works from one container but not another; a disk-pressure event makes a later deployment fail; a pipeline deploys an old artifact despite a successful build; a Terraform plan reveals unexpected destructive changes; or a rollback restores the service but leaves a database migration incompatible. Each variant should include enough evidence to support a reasoned diagnosis and a safe response.

### Final DevOps report

Show how the candidate investigated, which signals supported the diagnosis, what change was made, whether the service was verified or safely rolled back, how secrets and permissions were handled, and whether the handover is reproducible. Do not score only on whether the final deployment succeeds; safe diagnosis, a justified rollback, and clear operational communication can be stronger evidence than a lucky fix.

## Shared Mindfries platform behavior across both flows

- **One platform, role-specific blueprints:** FDE and DevOps use the same session, CLI, floating workspace, evidence timeline, evaluation engine, and report infrastructure. The platform changes task assets, enabled tabs, staged prompts, checks, and rubric by role.
- **Visible candidate journey:** show the current stage, task goal, permitted tools, deliverables, time guidance, and submission status. Avoid revealing hidden evaluator checks or the answer key.
- **Stateful floating tabs:** preserve the candidate's tab state across stages and share only the assessment context that the candidate is told is shared. Private notes do not silently become submitted evidence.
- **Adaptive but fair progression:** a follow-up can deepen a demonstrated skill or fill an evidence gap. Do not interpret lack of exposure as lack of ability, and do not change the task in a way that makes candidate results incomparable without recording the variant.
- **Evidence before inference:** link each scored claim to artifacts, checks, or observed task behavior. Keep score, evidence confidence, and coverage separate; disclose missing or failed infrastructure that could have affected the result.
- **Safe sandbox by default:** use synthetic or sanitized data, scoped short-lived credentials, disposable infrastructure, and no real production changes. Optional GitHub, CI, or deployment access must be explicitly authorized and relevant to the role.
- **Human-reviewed outcome:** the report supports hiring decisions rather than making an opaque automatic decision. Reviewers can inspect evidence, add notes, and record a decision with rationale.
