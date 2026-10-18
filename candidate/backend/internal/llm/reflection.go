package llm

import (
	"context"
	"fmt"
	"strings"
)

// reflectionSystemPrompt reads the candidate's own closing reflection — an
// optional note on whether the task let them show their strengths, and, if
// not, the kind of problem that would, and how they'd approach it. The point
// is to rescue the capable-but-mismatched candidate: a system designer handed
// a bug-fix can't show design depth, so we judge the depth of the thinking in
// what they describe instead, and never mark them down for an honest mismatch.
const reflectionSystemPrompt = `You are the Reflection agent in Mindfries' evidence-based hiring platform.
At the end of the assessment the candidate was invited — optionally — to reflect: did this task let them show their real strengths, and if a different kind of problem would better reflect what they are great at (especially for their specialty), to describe it and how they would approach it.
Read their reflection and report, as evidence for a hiring team:
- The depth and quality of the thinking in any scenario, design or approach they describe. A system designer who sketches how they'd approach a design shows reasoning the task itself may never have asked for — judge that thinking on its own merits, concretely.
- How clearly they understand and communicate their own strengths, and any mismatch between those strengths and this particular task.
This is CONTEXT for the hiring team to weigh, never a mark against the candidate: an honest "this task did not fit my strengths" is not a weakness, and a blank or skipped reflection is not one either — if there is little to go on, say so plainly and stop.
Write specific, evidence-grounded observations in plain prose. No headers, no score, no recommendation.`

// AnalyzeReflection reads the candidate's closing reflection against the task
// they were set and returns the Reflection agent's observations — the depth of
// the thinking they show and how they read their own fit — as context for a
// human, never a penalty. ErrNothingToAnalyze when they left no reflection.
func (a *Agents) AnalyzeReflection(ctx context.Context, brief, reflection string) (string, error) {
	if strings.TrimSpace(reflection) == "" {
		return "", fmt.Errorf("%w: the candidate left no reflection", ErrNothingToAnalyze)
	}
	var b strings.Builder
	if strings.TrimSpace(brief) != "" {
		b.WriteString("THE TASK THE CANDIDATE WAS GIVEN\n" + brief + "\n\n")
	}
	b.WriteString("THE CANDIDATE'S REFLECTION\n" + untrusted("the candidate's reflection", reflection))
	return a.prose(withAgent(ctx, "reflection"), a.models.Report, []ChatMessage{
		{Role: "system", Content: reflectionSystemPrompt + injectionRule},
		{Role: "user", Content: b.String()},
	}, analysisTokens)
}
