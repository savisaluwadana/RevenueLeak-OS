# Local development

Use Go 1.23 or newer. No backend or UI packages are needed: the application uses Go's standard library and browser JavaScript.

```bash
./start-demo.sh
```

Open http://127.0.0.1:8080. This seeds synthetic records and writes the ignored `data/revenueleak.json` file. Use another port by running the server directly:

```bash
DEMO_MODE=true ADDR=127.0.0.1:8081 go run ./cmd/server
```

For a private workspace, copy `.env.example` to `.env`, replace the password and session secret, and run `docker compose up --build`. Compose loads that file; `go run` and the demo launcher do not implicitly load `.env`.

```bash
go test -race ./...
go vet ./...
go build -o revenueleak ./cmd/server
node --check web/app.js
gofmt -w cmd internal
```

For the browser smoke suite (optional locally, required in CI), use Node 24, then:

```bash
npm ci
npx playwright install chromium
go build -o /tmp/revenueleak-next ./cmd/server
npm run test:browser
```

Set `REVENUELEAK_TEST_BINARY` if the compiled server is elsewhere. The suite creates an isolated temporary datastore and synthetic credentials, verifies desktop/mobile navigation, evidence downloads, review invalidation and restart persistence, then removes its test data. Node dependencies are test tooling only; the app still runs with Go alone. CI also builds the Docker image, validates Compose and invokes the executable demo launcher. After changing financial rules, add explicit expected-output fixtures for the changed contract behavior. Test unchanged cases, changed evidence, invalid records, persistence rollback and unauthorized exports.

For manual QA: run a scan, inspect Saved runs, download a JSON bundle, change a usage CSV, then inspect both the new result and the old run. The old export must remain identical; reviewed cases whose evidence changed must reopen. Restart the server and confirm history persists. Use synthetic data only for repository test fixtures.

Make development changes on a feature branch and open a PR. Keep roadmap and supported contract limitations accurate; a passing CI run is not production or finance sign-off.
