package git

// DeploymentPlan is the cumulative record of promotion decisions made
// across the HU-003..HU-006 flow: which commits were selected (HU-003),
// and — appended by later phases as their own screens confirm — the
// destination branch/sandbox/test level (HU-004), the created promotion
// branch (HU-005), and generated delta paths (HU-006). HU-003 only
// populates Ticket and SelectedCommits; later phases add fields without
// breaking this one, since a struct literal by field name is stable across
// additions.
type DeploymentPlan struct {
	// Ticket is the ticket the selection was discovered for.
	Ticket string
	// SelectedCommits holds exactly the user's chosen commits, in the same
	// order Service.Discover produced them (topological, earliest first) —
	// the "ordered chosen set" HU-006's cherry-pick engine consumes
	// directly.
	SelectedCommits []DiscoveredCommit
}

// GenerateDeploymentPlan produces a preliminary DeploymentPlan from a
// confirmed commit selection (HU-003 AC: "el usuario confirma una
// seleccion valida, entonces se genera un DeploymentPlan preliminar").
// It first runs ValidateSelection, so confirming with zero selected
// commits is blocked here too (ErrEmptySelection), never producing a
// half-valid plan.
func GenerateDeploymentPlan(ticket string, items []CommitSelectionItem) (DeploymentPlan, error) {
	if err := ValidateSelection(items); err != nil {
		return DeploymentPlan{}, err
	}

	var selected []DiscoveredCommit
	for _, item := range items {
		if item.Selected {
			selected = append(selected, item.DiscoveredCommit)
		}
	}

	return DeploymentPlan{
		Ticket:          ticket,
		SelectedCommits: selected,
	}, nil
}
