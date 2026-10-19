# Changelog

The update window shows the section of the version it offers. Every release
needs a `## <version>` section here, or its build fails.

## Unreleased

- Profiles: CCGuanjia finds `~/.claude` and every `~/.claude-*` folder that
  `CLAUDE_CONFIG_DIR` uses, with each one's account, plan and sign-in, read
  from the local files only.
- New profiles link skills, plugins, hooks, `CLAUDE.md`, `settings.json`,
  the status line, commands and agents to `~/.claude`, each one a switch
  you can turn off later. A file or folder of the profile's own is never
  replaced.
- Start a profile with `cc-<name>` from the shell file, **Open in
  Terminal**, or `ccgj run <name>`. Each one runs on its own sign-in, with
  the `ANTHROPIC_*` variables that would point it elsewhere cleared.
  Skipping permission prompts is a per-profile switch, off by default.
- Copy MCP servers from one profile to another. Only the `mcpServers` key of
  `.claude.json` changes.
- Overview: tokens, sessions and models from the transcripts over 24 hours,
  7 days or 30 days, what each folder takes by type and by project, and how
  Claude Code is installed.
- Install: the native installer, npm, Homebrew or Bun, with update and
  reinstall through each one's official command, and removal of the native
  installer's old versions that are neither current nor running.
- Cleanup, in the order of `clean-claude.ts`: copy the configuration out,
  archive `projects/` and `file-history/`, verify the archives, and only
  then edit `settings.json` and delete. Signing out is optional and comes
  last.
- The `ccgj` and `ccguanjia` commands: `list`, `run`, `use`, `default`,
  `dir`, `new` and `open`.
- English and Simplified Chinese.
