local wezterm = require 'wezterm'
local mux = wezterm.mux
local act = wezterm.action

wezterm.on("gui-startup", function(cmd)
  local tab, pane, window = mux.spawn_window(cmd or {})
  window:gui_window():maximize()
end)
wezterm.on('open-uri', function(window, pane, uri)
  local start, match_end = uri:find 'mailto:'
  if start == 1 then
    local recipient = uri:sub(match_end + 1)
    window:perform_action(
      act.SpawnCommandInNewWindow {
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
  automatically_reload_config = false,
  -- PROGRAMS ===============================================================
  -- https://wezfurlong.org/wezterm/config/launch.html
  --default_prog = { '/home/kiyama/.cargo/bin/zellij','a', '-c' },

  -- FONT ===================================================================
  -- https://wezfurlong.org/wezterm/config/fonts.html
  font = wezterm.font_with_fallback {
    {family="PlemolJP Console NF", weight="Medium"},
  -- 'Plemol JP Console NF'
    'JetBrains Mono',
  },

  -- KEYS ===================================================================
  -- https://wezfurlong.org/wezterm/config/keys.html
  leader = { key = 'a', mods = 'SUPER', timeout_milliseconds = 1000 },
  disable_default_key_bindings = true,
  keys = {
    { key = 'V', mods = 'CTRL', action = act.PasteFrom 'Clipboard' },
  },

  -- MOUSE ==================================================================
  -- https://wezfurlong.org/wezterm/config/mouse.html
  disable_default_mouse_bindings = true,
  mouse_bindings = {
    -- Change the default click behavior so that it only selects
    -- text and doesn't open hyperlinks
    {
      event = { Up = { streak = 1, button = 'Left' } },
      mods = 'NONE',
      action = act.CompleteSelection 'PrimarySelection',
    },

    -- and make CTRL-Click open hyperlinks
    {
      event = { Up = { streak = 1, button = 'Left' } },
      mods = 'CTRL|SHIFT',
      action = act.OpenLinkAtMouseCursor,
    },
    -- mouse wheel scroll
    {
      event = { Down = { streak = 1, button = { WheelUp = 1 } } },
      mods = 'NONE',
      action = act.ScrollByCurrentEventWheelDelta,
    },
    {
      event = { Down = { streak = 1, button = { WheelDown = 1 } } },
      mods = 'NONE',
      action = act.ScrollByCurrentEventWheelDelta,
    },
    -- NOTE that binding only the 'Up' event can give unexpected behaviors.
    -- Read more below on the gotcha of binding an 'Up' event only.
  },
  -- default_cursor_style = 'BlinkingBlock',

  -- COLOR ==================================================================
  -- https://wezfurlong.org/wezterm/config/appearance.html
  window_background_opacity = 0.75,
  colors = {
    visual_bell = '#202020',
  },

  -- DESIGN =================================================================
  -- hide_tab_bar_if_only_one_tab = true,
  adjust_window_size_when_changing_font_size = false,
  -- Enable the scrollbar.
  -- It will occupy the right window padding space.
  -- If right padding is set to 0 then it will be increased
  -- to a single cell width
  enable_scroll_bar = true,

  -- LINK ===================================================================
  hyperlink_rules = {
    {
      regex = [[\b(ipfs:|ipns:|magnet:|mailto:|gemini:|gopher:|https:|http:|news:|file:|git:|ssh:|ftp:)//[^\u0000-\u001F\u007F-\u009F<>\\s{-}\\^⟨⟩`]+\b]],
      format = '$0'
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

  -- BELL ===================================================================
  visual_bell = {
    fade_in_function = 'EaseIn',
    fade_in_duration_ms = 150,
    fade_out_function = 'EaseOut',
    fade_out_duration_ms = 150,
  },
  -- ========================================================================
  use_ime = true,
}
