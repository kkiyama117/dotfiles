local wezterm = require 'wezterm'
local mux = wezterm.mux

wezterm.on("gui-startup", function(cmd)
  local tab, pane, window = mux.spawn_window(cmd or {})
  window:gui_window():maximize()
end)
wezterm.on('open-uri', function(window, pane, uri)
  local start, match_end = uri:find 'mailto:'
  if start == 1 then
    local recipient = uri:sub(match_end + 1)
    window:perform_action(
      wezterm.action.SpawnCommandInNewWindow {
        args = { 'mutt', recipient },
      },
      pane
    )
    -- prevent the default action from opening in a browser
    return false
  end
  -- otherwise, by not specifying a return value, we allow later
  -- handlers and ultimately the default action to caused the
  -- URI to be opened in the browser
end)

return {

  default_prog = { '/home/kiyama/.cargo/bin/zellij' },
  -- FONT ===================================================================
  font = wezterm.font('JetBrains Mono'),
 -- font = wezterm.font('JetBrains Mono', { weight = 'Bold', italic = true }),
  window_background_opacity = 0.75,

  -- DESIGN =================================================================
  hide_tab_bar_if_only_one_tab = true,
  adjust_window_size_when_changing_font_size = false,
  use_ime = true,

  -- KEYS ===================================================================
  leader = { key = 'a', mods = 'SUPER', timeout_milliseconds = 1000 },
  disable_default_key_bindings = true,
  keys = {
    { key = 'V', mods = 'CTRL', action = wezterm.action.PasteFrom 'Clipboard' },
  },

  -- LINK ===================================================================
  hyperlink_rules = {
    -- Linkify things that look like URLs and the host has a TLD name.
    -- Compiled-in default. Used if you don't specify any hyperlink_rules.
    {
      regex = '\\b\\w+://[\\w.-]+\\.[a-z]{2,15}\\S*\\b',
      format = '$0',
    },

    -- linkify email addresses
    -- Compiled-in default. Used if you don't specify any hyperlink_rules.
    {
      regex = [[\b\w+@[\w-]+(\.[\w-]+)+\b]],
      format = 'mailto:$0',
    },

    -- file:// URI
    -- Compiled-in default. Used if you don't specify any hyperlink_rules.
    {
      regex = [[\bfile://\S*\b]],
      format = '$0',
    },

    -- Linkify things that look like URLs with numeric addresses as hosts.
    -- E.g. http://127.0.0.1:8000 for a local development server,
    -- or http://192.168.1.1 for the web interface of many routers.
    {
      regex = [[\b\w+://(?:[\d]{1,3}\.){3}[\d]{1,3}\S*\b]],
      format = '$0',
    },

    -- Make task numbers clickable
    -- The first matched regex group is captured in $1.
    {
      regex = [[\b[tT](\d+)\b]],
      format = 'https://example.com/tasks/?t=$1',
    },

    -- Make username/project paths clickable. This implies paths like the following are for GitHub.
    -- ( "nvim-treesitter/nvim-treesitter" | wbthomason/packer.nvim | wez/wezterm | "wez/wezterm.git" )
    -- As long as a full URL hyperlink regex exists above this it should not match a full URL to
    -- GitHub or GitLab / BitBucket (i.e. https://gitlab.com/user/project.git is still a whole clickable URL)
    {
      regex = [[["]?([\w\d]{1}[-\w\d]+)(/){1}([-\w\d\.]+)["]?]],
      format = 'https://www.github.com/$1/$3',
    },
  },
  -- ========================================================================
}
