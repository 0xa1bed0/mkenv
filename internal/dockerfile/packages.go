package dockerfile

type PackageSpec struct {
	Name string
	Meta map[string]string
}

func (ps PackageSpec) Clone() PackageSpec {
	return PackageSpec{
		Name: ps.Name,
		Meta: copyMap(ps.Meta),
	}
}

// PackageRequest is an abstract package reference.
type PackageRequest struct {
	Reason string
	Packages []PackageSpec
}

func (pr PackageRequest) Clone() PackageRequest {
	specs := make([]PackageSpec, len(pr.Packages))
	for i, spec := range pr.Packages {
		specs[i] = spec.Clone()
	}

	return PackageRequest{ Reason: pr.Reason, Packages: specs }
}

// PackageManager expands abstract PackageRequests into concrete root RUN steps.
type PackageManager interface {
	Name() string
	Install(pkgs []PackageSpec) []Command 
}

// ExpandPackages asks the system brick to convert requests to concrete steps.
func (plan *BuildPlan) expandPackages() {
	if plan == nil {
		return
	}
	if plan.system == nil {
		return
	}

	mgr := plan.system.PackageManager()
	if mgr == nil {
		return
	}
	if len(plan.packages) == 0 {
		return
	}

	steps := mgr.Install(plan.packages)
	if len(steps) > 0 {
		plan.rootRun = append(plan.rootRun, steps...)
	}
}
