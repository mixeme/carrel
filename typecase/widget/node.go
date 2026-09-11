package widget

// Node is any widget rendered as a fragment of markup.
type Node interface {
	Render() string
}

func renderNodes(nodes []Node) string {
	var out string
	for _, n := range nodes {
		out += n.Render()
	}
	return out
}

// ActionsAsNodes converts bar actions for []Node fields.
func ActionsAsNodes(actions []Action) []Node {
	if len(actions) == 0 {
		return nil
	}
	out := make([]Node, len(actions))
	for i, a := range actions {
		out[i] = a
	}
	return out
}
