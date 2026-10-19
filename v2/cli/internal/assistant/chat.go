// Package assistant is `mindfries agent`: the coding and research assistant
// in the candidate's terminal (v2/PLAN.md Stage 1b). The model runs on the
// server under the assessment's policy; this side shows replies as they
// stream, carries out tool calls inside the task folder with the
// candidate's approval, and records every edit it applies.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/ui"
)

// Chat is one `mindfries agent` session in the terminal.
type Chat struct {
	Client *api.Client
	Exec   *Executor
	UI     *ui.UI
	Out    io.Writer
	Policy api.Policy
	// ConvPath remembers the conversation between runs.
	ConvPath string
	// NewConversation starts afresh instead of carrying on.
	NewConversation bool

	conv   string
	budget *api.Budget
}

var modeText = map[string]string{
	api.ModeExplainOnly: "explain only: it explains and researches, but won't write code or run commands",
	api.ModeCodeAllowed: "it can propose code changes and commands; nothing happens without your yes",
}

// Run reads the candidate's messages until /exit or end of input.
func (c *Chat) Run(ctx context.Context) error {
	if c.Policy.AssistantMode == api.ModeOff {
		fmt.Fprintln(c.Out, "The assistant is off for this assessment.")
		return nil
	}
	c.budget = c.Policy.AssistantBudget
	if !c.NewConversation {
		c.loadConv()
	}
	fmt.Fprintf(c.Out, "mindfries agent: %s.\n", modeText[c.Policy.AssistantMode])
	if c.budget != nil {
		fmt.Fprintf(c.Out, "Budget: %d of %d tokens left.\n", c.budget.TokensRemaining, c.budget.TokensTotal)
	}
	fmt.Fprintln(c.Out, "Everything you ask and everything it does is part of the assessment record.")
	fmt.Fprintln(c.Out, "Type a message (end a line with \\ to continue it). /help for commands, /exit to leave.")
	for {
		msg, ok := c.read()
		if !ok {
			fmt.Fprintln(c.Out)
			return nil
		}
		switch strings.TrimSpace(msg) {
		case "":
			continue
		case "/exit", "/quit":
			return nil
		case "/help":
			fmt.Fprintln(c.Out, "/new     start a new conversation\n/budget  show the tokens left\n/exit    leave (your session carries on)")
			continue
		case "/new":
			c.conv = ""
			c.saveConv()
			fmt.Fprintln(c.Out, "Started a new conversation.")
			continue
		case "/budget":
			c.printBudget()
			continue
		}
		if err := c.turn(ctx, msg); err != nil {
			if p, ok := api.AsProblem(err); ok {
				switch p.Code {
				case api.CodeBudgetExhausted:
					fmt.Fprintln(c.Out, "The assistant's budget for this session is used up. You can keep working without it.")
					continue
				case api.CodeAssistantOff:
					fmt.Fprintln(c.Out, "The assistant is off for this assessment.")
					return nil
				}
				if api.IsAuthFailure(err) || p.Code == api.CodeSessionNotActive {
					return errors.New("this assessment session has ended")
				}
			}
			fmt.Fprintf(c.Out, "\nThat didn't go through (%v). Try again.\n", err)
		}
	}
}

func (c *Chat) read() (string, bool) {
	fmt.Fprint(c.Out, "\nyou> ")
	var parts []string
	for {
		line, ok := c.UI.Line()
		if !ok {
			return strings.Join(parts, "\n"), len(parts) > 0
		}
		if strings.HasSuffix(line, "\\") {
			parts = append(parts, strings.TrimSuffix(line, "\\"))
			fmt.Fprint(c.Out, "...> ")
			continue
		}
		parts = append(parts, line)
		return strings.Join(parts, "\n"), true
	}
}

func (c *Chat) printBudget() {
	if c.budget == nil {
		fmt.Fprintln(c.Out, "No budget is set for this session.")
		return
	}
	fmt.Fprintf(c.Out, "%d of %d tokens left.\n", c.budget.TokensRemaining, c.budget.TokensTotal)
}

// turn sends one message and follows the turn through any tool calls.
func (c *Chat) turn(ctx context.Context, msg string) error {
	st, err := c.Client.StartTurn(ctx, api.AssistantTurnRequest{ConversationID: c.conv, Message: msg})
	if err != nil {
		return err
	}
	fmt.Fprint(c.Out, "\nagent> ")
	for {
		turnID, calls, err := c.follow(st)
		st.Close()
		if err != nil {
			return err
		}
		if len(calls) == 0 {
			return nil
		}
		var results []api.ToolResult
		for _, call := range calls {
			results = append(results, c.Exec.Execute(ctx, call))
		}
		st, err = c.Client.SendToolResults(ctx, turnID, api.ToolResults{Results: results})
		if err != nil {
			return err
		}
		fmt.Fprint(c.Out, "\nagent> ")
	}
}

// follow prints a stream and returns the calls it asked for, if any.
func (c *Chat) follow(st *api.Stream) (string, []api.ToolCall, error) {
	var calls []api.ToolCall
	for {
		ev, err := st.Next()
		if err == io.EOF {
			return "", nil, errors.New("the reply was cut off")
		}
		if err != nil {
			return "", nil, err
		}
		switch ev.Type {
		case api.StreamTurnStarted:
			if ev.ConversationID != "" && ev.ConversationID != c.conv {
				c.conv = ev.ConversationID
				c.saveConv()
			}
			if ev.Budget != nil {
				c.budget = ev.Budget
			}
		case api.StreamTextDelta:
			fmt.Fprint(c.Out, ev.Text)
		case api.StreamResearchSource:
			title := ev.Title
			if title == "" {
				title = ev.URL
			}
			fmt.Fprintf(c.Out, "\n  [source] %s (%s)\n", title, ev.URL)
		case api.StreamResearchRefused:
			fmt.Fprintf(c.Out, "\n  [research refused: %s isn't an allowed source for this assessment]\n", ev.Domain)
		case api.StreamToolCall:
			if ev.Call != nil {
				calls = append(calls, *ev.Call)
			}
		case api.StreamCallsPending:
			fmt.Fprintln(c.Out)
			return ev.TurnID, calls, nil
		case api.StreamTurnCompleted:
			if ev.Budget != nil {
				c.budget = ev.Budget
				fmt.Fprintf(c.Out, "\n(%d tokens left)\n", ev.Budget.TokensRemaining)
			} else {
				fmt.Fprintln(c.Out)
			}
			return ev.TurnID, nil, nil
		case api.StreamError:
			if ev.Code == api.CodeBudgetExhausted {
				return "", nil, &api.Problem{Code: api.CodeBudgetExhausted, Title: ev.Message, Status: 429}
			}
			return "", nil, fmt.Errorf("the assistant stopped: %s", orText(ev.Message, ev.Code))
		}
	}
}

func orText(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type convFile struct {
	ConversationID string `json:"conversation_id"`
}

func (c *Chat) loadConv() {
	b, err := os.ReadFile(c.ConvPath)
	if err != nil {
		return
	}
	var f convFile
	if json.Unmarshal(b, &f) == nil {
		c.conv = f.ConversationID
	}
}

func (c *Chat) saveConv() {
	if c.ConvPath == "" {
		return
	}
	b, _ := json.Marshal(convFile{ConversationID: c.conv})
	_ = state.WriteFileAtomic(c.ConvPath, b, 0o600)
}
