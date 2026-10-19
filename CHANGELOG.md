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
- A new look: cards with a hairline border and a soft shadow, quieter
  titles, and charts of rounded columns that say what each one is under
  the pointer.
- Motion that stays out of the way: cards come in one after another as a
  page opens, the sidebar's highlight and the range control's thumb glide
  to what you choose, numbers roll to their new values, and charts grow in.
  Reduce Motion in System Settings turns it off.
- Overview: when you work, as a heatmap of the hours of the week with the
  busiest one; tokens by project; and the recent sessions, each with
  **Resume**, which opens the terminal in its folder and resumes it with
  the profile, and a button copying its ID.
- Command palette (⌘K): go to a page or a profile, start a profile in the
  terminal, make one, refresh or check for updates, by typing a few
  letters.
- Keyboard shortcuts: ⌘N for a new profile and ⌥⌘↓ / ⌥⌘↑ for the next or
  previous profile, listed in Settings.
- Show a profile's folder in Finder from its page.
- Placeholders that glow softly while something is read, and the refresh
  button turns until everything is.
- Faster: a profile's `.claude.json` is read once as it loads instead of
  in every frame, and the free space of the backup folder every few
  seconds.
- The `cleanupPeriodDays` field of Cleanup no longer runs past its card.
