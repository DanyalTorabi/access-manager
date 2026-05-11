## Plan: Add CORS Whitelisting and Development Defaults

To address the CORS issue in the local development environment, we will implement a mechanism to whitelist origins. For development, the server will allow all origins by default.

**Steps**
1. **Configuration Update**
   - Add a new configuration option for `CORS_ALLOWED_ORIGINS` in the configuration file (e.g., `config.example.yaml`).
   - Document the new option in the `README.md` under the configuration section.

2. **Development Defaults**
   - Set the default behavior in the development environment to allow all origins.
   - Ensure this behavior is overridden in production by the `CORS_ALLOWED_ORIGINS` configuration.

3. **Middleware Implementation**
   - Implement a CORS middleware in `internal/api` to handle the `Access-Control-Allow-Origin` header.
   - Use the `CORS_ALLOWED_ORIGINS` configuration to determine the allowed origins.
   - For development, set the middleware to allow all origins.

4. **Testing**
   - Add unit tests for the middleware to verify:
     - Correct handling of allowed origins.
     - Default behavior in development.
     - Rejection of disallowed origins in production.
   - Add integration tests to ensure the middleware works with the server setup.

5. **Plan File Creation**
   - Create a new plan file under `plan/phase-1/` (e.g., `T45-cors-whitelisting.md`) with the above details.
   - Reference the GitHub issue number in the plan file.

**Relevant files**
- `go/internal/api/middleware.go` — Add the CORS middleware.
- `go/config.example.yaml` — Add the `CORS_ALLOWED_ORIGINS` option.
- `go/README.md` — Document the new configuration.
- `plan/phase-1/T45-cors-whitelisting.md` — Save the plan file.

**Verification**
1. Run the server locally and verify that all origins are allowed in development.
2. Deploy to a staging environment and verify that only whitelisted origins are allowed.
3. Run the test suite to ensure all tests pass.

**Decisions**
- Development will allow all origins by default.
- Production will strictly enforce the `CORS_ALLOWED_ORIGINS` configuration.

**Further Considerations**
1. Should we log a warning if `CORS_ALLOWED_ORIGINS` is not set in production?
2. Should we provide a default set of origins for production if none are specified?