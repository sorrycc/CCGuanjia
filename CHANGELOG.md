# Changelog

The update window shows the section of the version it offers. Every release
needs a `## <version>` section here, or its build fails.

## 0.1.1

- **Claude Settings** (`⌘4`) edits each profile's `settings.json` in a form or as JSON.
- The form covers the model, effort, permission rules and mode, environment variables and plugins.
- The JSON tab is a code editor that highlights, checks known keys and underlines errors.
- Each save shows its diff, stops if the file changed since it was read, and backs it up first.
- A shared `settings.json` says which profiles a change reaches, and **Detach** makes it the profile's own.
- **Compare** copies chosen keys from another profile's `settings.json`.
- A changed value shows a dot and a revert button, and a bar offers **Discard** and **Review**.
- `⌘S` opens the review, which shows the changes key by key or as a diff of lines.
- The review strikes through removed values and colors the new ones.
- The JSON editor marks the lines changed since the file was read in its gutter.
- A strip at the top shows the file, its link and who shares it, with buttons to show them in Finder.
- Pointing at a setting shows its key in `settings.json`.
- Clicking a variable loads it into the fields to edit, and secret values show on request.
- A list no longer adds the same item twice.
- The bar of unsaved changes steps aside while its review is open.
- Install lines up the **Use** buttons of native versions and balances its two columns.
- Cleanup names the transcript setting in words and fits the window without scrolling.
- Copy buttons for launchers, session IDs and account fields turn to a check for a moment.
- The command palette opens a profile's `settings.json` and shows its shortcuts on keys.
- Overview's bars of tokens by model and by project thicken under the pointer.
- Settings slides in the line the shell file adds to `~/.zshrc`, with a button to copy it.
- Pages keep a readable width in a wide window and stack their columns in a narrow one.
- Controls, menus and dialogs take the window's colors and rounding.
- Dialogs and the command palette rise in as the window dims behind them.
- The header shows a hairline once the page scrolls under it.
- Overview's large numbers count up as their cards come in.
- Activity by Hour fills its card, and an hour without replies shows as a faint cell.
- Recent Sessions shows **Show in Finder** and **Copy ID** only on the row under the pointer.
- Sidebar icons grow under the pointer, and cards that open a page press in when clicked.
- Storage by Type highlights a part of its ring and its row under the pointer, with name and size.
- The tiles of Storage by Project and the parts of the Storage bar name themselves under the pointer.
- Loading placeholders shimmer in one wave across the page.
- Cleanup fades unchecked items, and removed list rows collapse instead of vanishing.
- In the light appearance, dialogs and the command palette dim the window less.
- Install Version highlights versions under the pointer.

## 0.1.0

- CCGuanjia finds `~/.claude` and every `~/.claude-*` profile, with its account, plan and sign-in.
- New profiles link skills, plugins, hooks, `CLAUDE.md` and more to `~/.claude`, each one a switch.
- Linking never replaces a file or folder of the profile's own.
- Start a profile with `cc-<name>`, **Open in Terminal**, or `ccgj run <name>`, on its own sign-in.
- Skipping permission prompts is a per-profile switch, off by default.
- Copy MCP servers from one profile to another; only `mcpServers` in `.claude.json` changes.
- Overview shows tokens, sessions and models over 24 hours, 7 days or 30 days.
- Overview shows what each folder takes by type and by project, and how Claude Code is installed.
- Overview shows all profiles together by default, and the profile menu narrows it to one.
- Install updates and reinstalls Claude Code through the native installer, npm, Homebrew or Bun.
- Install removes the native installer's old versions that are neither current nor running.
- Install a chosen version of Claude Code, except through Homebrew.
- **Keep this version** sets `DISABLE_AUTOUPDATER` so the chosen version stays.
- A native version already downloaded has **Use** to switch to it.
- Cleanup backs up and verifies the configuration and transcripts before it deletes anything.
- Signing out is an optional last step of Cleanup.
- The `ccgj` and `ccguanjia` commands: `list`, `run`, `use`, `default`, `dir`, `new` and `open`.
- English and Simplified Chinese.
- Cards with a hairline border and soft shadow, and charts that label each column under the pointer.
- Pages, highlights, numbers and charts animate in, unless **Reduce Motion** is on.
- Overview shows a heatmap of the hours of the week you work, with the busiest one.
- Overview shows tokens by project.
- Recent Sessions has **Resume**, which resumes a session in its folder, and a button to copy its ID.
- The command palette (`⌘K`) goes to pages and profiles, starts or makes a profile, and refreshes.
- `⌘N` makes a new profile and `⌥⌘↓` / `⌥⌘↑` go to the next or previous one.
- Show a profile's folder in Finder from its page.
- Each profile has an avatar of its initial in its own color, and the default profile is marked ★.
- A green dot marks a profile active in the last 5 minutes, and live sessions show **Live**.
- Overview updates minute by minute while the window is in front.
- Tokens and Sessions show how they changed from the period before.
- Overview's Profiles card shows each profile's tokens, and a click shows that profile.
- Recent Sessions shows 6 sessions, and **Show all** the 20 latest, each with a Finder button.
- The command palette resumes recent sessions, copies launchers and shows folders in Finder.
- The command palette scrolls with the arrow keys past its 8 visible rows.
- When a newer Claude Code is out, the sidebar says so and Install offers **What's New**.
- Cleanup shows the folder's size now and after the cleanup, with a bar of what it removes.
- Cards that open a page rise a little under the pointer.
- Bringing the window to the front reuses folder sizes for 2 minutes, and Refresh measures at once.
- Placeholders glow while something loads, and the refresh button spins until it is done.
- The `cleanupPeriodDays` field of Cleanup no longer runs past its card.
