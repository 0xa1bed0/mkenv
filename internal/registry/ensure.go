package registry

// This reference forces the presence of the generated file at build time.
// If zz_generated_imports.go is missing or its version mismatches, build fails.
const _ = requiresGeneratedV1

const requiresGeneratedV1 = GeneratedRegistryVersion // must exist in generated file
