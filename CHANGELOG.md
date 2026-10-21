# Changelog

The update window shows the section of the version it offers. Every release
needs a `## <version>` section here, or its build fails.

## Unreleased

- Claude Settings (⌘4): edit each profile's `settings.json` in a form,
  with the model, effort, permission rules and mode, environment
  variables and plugins, or as JSON, which checks the types of the keys it
  knows. Each save shows its diff, stops when the file changed since it
  was read, and backs the file up first. A file shared through a link says
  which profiles a change reaches, **Detach** makes it the profile's own,
  and Compare copies chosen keys from another profile's file.

## 0.1.0

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
- Overview shows every profile together by default: each account with its
  plan and sign-in, tokens, sessions and storage summed, and recent sessions
  of all profiles, each resuming in its own. The profile menu narrows it to
  one.
- Install: the native installer, npm, Homebrew or Bun, with update and
  reinstall through each one's official command, and removal of the native
  installer's old versions that are neither current nor running.
- Install a chosen version of Claude Code: pick one of npm's recent versions
  or type one, and it runs the official command of the way it was installed
  (not Homebrew, whose cask has only the newest). **Keep this version** sets
  `DISABLE_AUTOUPDATER` in `~/.claude/settings.json` so it stays. A native
  version already downloaded has **Use** to switch to it.
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
- Each profile has an avatar of its initial in its own color, in the
  sidebar, Overview and the command palette, and the default profile of
  new shells is marked with ★.
- Live: a green dot breathes on a profile that wrote a transcript in the
  last 5 minutes, and Recent Sessions marks the sessions going on as
  **Live**. While the window is in front, Overview follows Claude Code
  minute by minute.
- Tokens and Sessions show how they changed from the 24 hours, 7 days or
  30 days before.
- Overview's Profiles card shows each profile's tokens over the range; a
  click shows that profile.
- Recent Sessions shows 6 sessions, and **Show all** the 20 latest, each
  with a button showing its folder in Finder.
- The command palette resumes the latest sessions, copies a profile's
  launcher and shows its folder in Finder. Its list now follows the arrow
  keys past the 8 rows it shows, and its highlight glides between rows.
- When a newer Claude Code is out, the sidebar says so, and Install has
  **What's New**, opening Claude Code's changelog.
- Cleanup shows the folder now and after the cleanup chosen, with a bar
  of what it removes that follows the check boxes.
- Cards that open a page rise a little under the pointer.
- The window coming to the front no longer measures the profile's whole
  folder again each time: a measure stays for 2 minutes, and Refresh
  measures at once.
- Placeholders that glow softly while something is read, and the refresh
  button turns until everything is.
- Faster: a profile's `.claude.json` is read once as it loads instead of
  in every frame, and the free space of the backup folder every few
  seconds.
- The `cleanupPeriodDays` field of Cleanup no longer runs past its card.
