# Changelog

The update window shows the section of the version it offers. Every release
needs a `## <version>` section here, or its build fails.

## 0.1.1

- Claude Settings (⌘4): edit each profile's `settings.json` in a form,
  with the model, effort, permission rules and mode, environment
  variables and plugins, or as JSON, which checks the types of the keys it
  knows. Each save shows its diff, stops when the file changed since it
  was read, and backs the file up first. A file shared through a link says
  which profiles a change reaches, **Detach** makes it the profile's own,
  and Compare copies chosen keys from another profile's file.
- Claude Settings looks after what you change: a dot and a revert button
  mark each value that differs from the file, a bar rises with Discard and
  Review while there are unsaved changes, and ⌘S opens the review. The
  JSON tab is a code editor with highlighting, the lines changed marked in
  its gutter and the error underlined where it is. The review shows the
  changes key by key or as a diff of the file's lines.
- Claude Settings is laid out anew: the file, what it links to and who
  shares it sit in a strip at the top, with Detach and buttons showing the
  file and its backups in Finder, and General comes first. Pointing at a
  setting shows its key in `settings.json`. A click on a variable puts it
  in the fields to change its value, the values of secret ones show on
  request, and an item already in a list is not added twice. The JSON
  editor takes the window's height, and the page works out its lists,
  variables, plugins, hooks and comparison once per edit.
- The review of Claude Settings shows each change as its key, with the
  value removed struck through and the value it becomes in color.
- Install lines up the Use buttons of the native versions, and Cleanup
  names the transcript setting in words, as Claude Settings does.
- Copying a launcher or a session's ID turns its button to a check for a
  moment.
- The command palette opens a profile's `settings.json` on Claude
  Settings, and shows its shortcuts on keys.
- Overview's bars of tokens by model and by project thicken under the
  pointer, and Settings shows the line the shell file adds to `~/.zshrc`
  sliding in as it turns on, with a button copying it.
- A profile's account fields show a button copying the value under the
  pointer.
- Cleanup puts the backup and its button under what it removes, so the
  page fits the window and Back Up and Clean shows without scrolling.
- In a wide window the pages keep a width that reads easily, centered
  under a header that lines up with them, rather than stretching across.
- In a narrow window the pages of two columns stack them in one, and
  Overview shows two columns, so notes and titles are not cut short.
- Buttons, fields, menus and dialogs take the window's own colors and
  rounding: in the light appearance fields are white with a hairline
  border, and dialogs sit on a card.
- Dialogs and the command palette rise into place as the window dims
  behind them, rather than appearing at once.
- The header shows a hairline once a page is scrolled under it.
- Overview's large numbers count up as their cards come in, and the bar
  of unsaved changes in Claude Settings steps aside while its review is
  open.
- Activity by Hour fills its card, and an hour without replies is a faint
  cell rather than a dark hole.
- Recent Sessions keeps Show in Finder and Copy ID quiet until the
  pointer is on their row, so that the list reads as its sessions.
- The sidebar's icons grow a little under the pointer, and a card that
  opens a page presses in as it is clicked.
- Overview with all profiles sums their storage once per measurement
  rather than on every frame.
- Storage by Type answers the pointer: a part of the ring, or its row in
  the list beside it, thickens as the others fade, and the middle of the
  ring shows its name and size. The tiles of Storage by Project and the
  parts of Storage's bar name themselves under the pointer, so that a
  tile too small for a label can still be read.
- What is still being read shows a soft light sweeping across its
  blocks, one wave over the page, drawn without building the window
  again.
- Install puts the installs on the PATH under Claude Code, so that its
  two columns come out about as long.
- Cleanup fades an item whose box is cleared, so that what goes stands
  out, and lines of a list, as a native version removed, collapse rather
  than vanish.
- In the light appearance dialogs and the command palette dim the window
  less, so that it stays clear behind them. The versions of Install
  Version light up under the pointer, and a note that changes with its
  switch fades across.

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
