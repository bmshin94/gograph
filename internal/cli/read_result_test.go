package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ozgurcd/gograph/internal/graph"
)

func TestContextAbsentRefusesInBothOutputModes(t *testing.T) {
	root := writeCLIParityGraph(t, &graph.Graph{})
	previous := jsonMode
	t.Cleanup(func() { jsonMode = previous; resetOutputGraph() })
	for _, machine := range []bool{false, true} {
		jsonMode = machine
		stdout, stderr, code := runCLIParityInDir(t, root, func() int { return runContext([]string{"Absent"}) })
		if code != 1 {
			t.Fatalf("missing context must refuse: code=%d stdout=%s stderr=%s", code, stdout, stderr)
		}
		if machine {
			var result Envelope
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if result.Error != "symbol 'Absent' not found" || result.Status != "error" || result.GraphState == nil || result.GraphState.ReadDiagnostic != result.Error {
				t.Fatalf("missing context lost refusal/provenance: %s", stdout)
			}
		} else if strings.TrimSpace(stderr) != "symbol 'Absent' not found" {
			t.Fatalf("human refusal differs: %q", stderr)
		}
	}
}
