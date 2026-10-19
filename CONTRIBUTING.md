# Contributing

## Development

```sh
scripts/dev.sh                                   # run CCGuanjia Dev with live reload
go test ./...                                    # unit tests and headless UI tests
go test -short ./...                             # what CI runs
go run ./cmd/ccguanjia cli list                  # the ccgj command, from source
CCGJ_SHOTS=build/shots go test -run TestShots ./cmd/ccguanjia   # draw every page, dark and light
go run ./tools/icongen                           # redraw resources/icon.png
go tool mygo generate                            # after editing locales/*.json
```

The UI tests run against a made-up home folder (`fixture` in
`cmd/ccguanjia/app_test.go`) with a Keychain in memory, so they never touch
your profiles. The dev app does read your real `~/.claude` folders.

Messages live in `locales/en.json` and `locales/zh-Hans.json`. Add every new
one to both.

## Releasing

Write the release notes under `## Unreleased` in `CHANGELOG.md` and commit them, then:

```sh
scripts/bump.sh patch    # or minor, major, or a version such as 0.2.0
```

It sets the version in `mygo.json`, renames `## Unreleased` to
`## <version>` in `CHANGELOG.md`, commits and tags `v<version>`, and pushes
`main` and the tag. The [Release workflow](.github/workflows/release.yml)
then builds the app for Apple silicon and Intel, signs and notarizes it,
uploads it with its signed updates, and publishes the release.

The workflow needs these repository secrets:

| Secret | What |
|---|---|
| `MYGO_UPDATER_PRIVATE_KEY` | `mygo-update.key` from `go tool mygo keygen`, whose public key is in `mygo.json` |
| `MACOS_CERTIFICATE` | the Developer ID Application certificate and its key, as a base64 `.p12` |
| `MACOS_CERTIFICATE_PASSWORD` | the password of the `.p12` |
| `APPLE_ID`, `APPLE_TEAM_ID` | the Apple ID of the developer account, and its team ID |
| `APPLE_APP_PASSWORD` | an app-specific password of the Apple ID, from account.apple.com |

Local builds stay signed ad hoc; only the workflow signs with the Developer ID.
