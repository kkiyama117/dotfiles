set,Path,C:\Python313\Scripts\;C:\Python313\;C:\Python312\Scripts\;C:\Python312\;C:\Program Files\OpenSSH\;C:\Windows\System32;C:\Windows;C:\Windows\System32\wbem;C:\Windows\System32\WindowsPowerShell\v1.0\;C:\Windows\System32\OpenSSH\;C:\Program Files\Tailscale\;C:\Program Files\WezTerm;C:\ProgramData\chocolatey\bin;C:\Program Files\Git\cmd;C:\Program Files\PowerShell\7-preview\preview;C:\Users\kkouh\.local\bin;C:\Program Files\gnuplot\bin;C:\Program Files\Calibre2\;C:\Program Files\Docker\Docker\resources\bin;C:\Program Files\Neovide\;C:\Program Files\Neovim\bin;C:\Program Files\PuTTY\;C:\Program Files (x86)\Gpg4win\..\GnuPG\bin;C:\Program Files\starship\bin\;C:\Program Files\PowerShell\7-preview;;C:\Users\kkouh\.local\bin;C:\Users\kkouh\.cargo\bin;C:\Users\kkouh\AppData\Local\Microsoft\WindowsApps;C:\Users\kkouh\AppData\Local\JetBrains\RustRover 2024.2.4\bin;C:\Users\kkouh\AppData\Local\JetBrains\Toolbox\scripts;C:\Program Files\wsl-ssh-pageant;C:\Users\kkouh\AppData\Local\Programs\Microsoft VS Code\bin;C:\Users\kkouh\AppData\Roaming\Programs\Zero Install;C:\Program Files\JetBrains\PyCharm 2024.3\bin;C:\Users\kkouh\AppData\Local\Microsoft\WindowsApps;C:\Users\kkouh\AppData\Local\Microsoft\WinGet\Packages\topgrade-rs.topgrade_Microsoft.Winget.Source_8wekyb3d8bbwe;C:\Program Files\Neovim\bin;C:\Users\kkouh\AppData\Local\Microsoft\WinGet\Links;C:\Program Files\Bitwarden\cli;C:\Users\kkouh\AppData\Local\Programs\nu\bin\

export-env {
  $env.MISE_SHELL = "nu"
  let mise_hook = {
    condition: { "MISE_SHELL" in $env }
    code: { mise_hook }
  }
  add-hook hooks.pre_prompt $mise_hook
  add-hook hooks.env_change.PWD $mise_hook
}

def --env add-hook [field: cell-path new_hook: any] {
  let old_config = $env.config? | default {}
  let old_hooks = $old_config | get $field --ignore-errors | default []
  $env.config = ($old_config | upsert $field ($old_hooks ++ [$new_hook]))
}

def "parse vars" [] {
  $in | from csv --noheaders --no-infer | rename 'op' 'name' 'value'
}

export def --env --wrapped main [command?: string, --help, ...rest: string] {
  let commands = ["deactivate", "shell", "sh"]

  if ($command == null) {
    ^"C:\\Users\\kkouh\\AppData\\Local\\Microsoft\\WinGet\\Links\\mise.exe"
  } else if ($command == "activate") {
    $env.MISE_SHELL = "nu"
  } else if ($command in $commands) {
    ^"C:\\Users\\kkouh\\AppData\\Local\\Microsoft\\WinGet\\Links\\mise.exe" $command ...$rest
    | parse vars
    | update-env
  } else {
    ^"C:\\Users\\kkouh\\AppData\\Local\\Microsoft\\WinGet\\Links\\mise.exe" $command ...$rest
  }
}

def --env "update-env" [] {
  for $var in $in {
    if $var.op == "set" {
      if $var.name == 'PATH' {
        $env.PATH = ($var.value | split row (char esep))
      } else {
        load-env {($var.name): $var.value}
      }
    } else if $var.op == "hide" {
      hide-env $var.name
    }
  }
}

def --env mise_hook [] {
  ^"C:\\Users\\kkouh\\AppData\\Local\\Microsoft\\WinGet\\Links\\mise.exe" hook-env -s nu
    | parse vars
    | update-env
}

