package version

// ImageSchemaVersion increments when Dockerfile generation changes require image rebuilds.
//
// Bump for:
//   - Dockerfile generation logic changes
//   - Label format changes
//   - User/permission structure changes
//   - Entrypoint/CMD format changes
//   - Cache volume path conventions change
//
// Don't bump for:
//   - CLI-only changes
//   - Bug fixes not affecting image content
const ImageSchemaVersion = 1

const ImageSchemaVersionLabel = "mkenv.image_schema_version"
