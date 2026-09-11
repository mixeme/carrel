package widget

// Labels holds consumer-supplied UI strings for widgets that would otherwise
// hard-code English. Set via SetLabels before rendering.
type Labels struct {
	Close          string
	Cancel         string
	Search         string
	Clear          string
	AllResults     string
	Recent         string
	Loading        string
	More           string
	Unavailable    string
	UnsavedChanges string
	Toggle         string
	Actions        string
	ClosePanel     string
	From           string
	To             string
	Main           string
	Breadcrumb     string
	BuildMissing   string
	Progress       string
	Rotate90       string
	Sources        string
	Apply          string
	StepOf         string // "Step %d of %d" with %d placeholders — use StepOfFunc instead
}

// StepOfFunc formats step progress; when nil, defaults to "Step N of M".
type stepOfFunc func(cur, total int) string

var (
	labels = defaultLabels()
	stepOf stepOfFunc
)

func defaultLabels() Labels {
	return Labels{
		Close:          "Close",
		Cancel:         "Cancel",
		Search:         "Search",
		Clear:          "Clear",
		AllResults:     "All results",
		Recent:         "Recent",
		Loading:        "Loading",
		More:           "More",
		Unavailable:    "unavailable",
		UnsavedChanges: "Unsaved changes",
		Toggle:         "Toggle",
		Actions:        "Actions",
		ClosePanel:     "Close panel",
		From:           "From",
		To:             "To",
		Main:           "Main",
		Breadcrumb:     "Breadcrumb",
		BuildMissing:   "Build information missing",
		Progress:       "Progress",
		Rotate90:       "Rotate 90°",
		Sources:        "Sources",
		Apply:          "Apply",
	}
}

// SetLabels replaces package UI strings for the current render pass.
func SetLabels(l Labels) {
	labels = l
}

// SetStepOfFunc sets the formatter for tc-steps progress text.
func SetStepOfFunc(fn stepOfFunc) {
	stepOf = fn
}

func lbl(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}

func formatStep(cur, total int) string {
	if stepOf != nil {
		return stepOf(cur, total)
	}
	return "Step " + itoa(cur) + " of " + itoa(total)
}
