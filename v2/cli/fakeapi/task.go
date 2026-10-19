package fakeapi

import "github.com/mindfries/v2/cli/internal/api"

// Task is what the fake serves as the task bundle.
type Task struct {
	Version       string
	Files         map[string]string // path -> content
	Locked        []string
	Executable    []string
	Runtimes      []api.Runtime
	SetupCommands []string
	CheckCommands []string
	Branch        string
	AuthorName    string
	AuthorEmail   string
}

// DemoTask is a small Python task.
func DemoTask() *Task {
	return &Task{
		Version: "demo-1.0.0",
		Files: map[string]string{
			"TASK.md":         "# Rate limiter\n\nImplement `allow(key)` in `limiter.py` so each key gets at most\n5 requests per rolling minute. Keep the tests in `test_limiter.py` passing.\n",
			"limiter.py":      "def allow(key):\n    raise NotImplementedError\n",
			"test_limiter.py": "from limiter import allow\n\n\ndef test_first_request_is_allowed():\n    assert allow(\"a\")\n",
			".gitignore":      "__pycache__/\n.venv/\n",
		},
		Locked:        []string{"TASK.md"},
		Runtimes:      []api.Runtime{{Name: "python", Version: ">=3.10", InstallURL: "https://www.python.org/downloads/"}},
		CheckCommands: []string{"python -m pytest -q"},
		Branch:        "mindfries/assessment",
		AuthorName:    "Candidate",
		AuthorEmail:   "candidate@assessment.mindfries.invalid",
	}
}
