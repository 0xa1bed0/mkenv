package bricksengine

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
	Reason   string
	Packages []PackageSpec
}

func (pr PackageRequest) Clone() PackageRequest {
	specs := make([]PackageSpec, len(pr.Packages))
	for i, spec := range pr.Packages {
		specs[i] = spec.Clone()
	}

	return PackageRequest{Reason: pr.Reason, Packages: specs}
}

// PackageManager expands abstract PackageRequests into concrete root RUN steps.
type PackageManager interface {
	Name() string
	Install(pkgs []PackageSpec) []Command
}
