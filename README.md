# HxTermux

HxTermux is a Termux setup with a responsive Bubble Tea installer and customization studio. It installs a curated shell environment, lets you choose a terminal palette and font, and builds the included HypexFetch system banner.

## Install

Inside Termux, run the one-line installer:

```sh
curl -fsSL https://raw.githubusercontent.com/H1p4zdev/HxTermux/main/install.sh | bash
```

It downloads HxTermux to `~/.local/share/hxtermux/source`, installs Git and Go if needed, and launches the setup wizard. You can also clone the repository and run `./install.sh` from its directory.

To build a compiled Termux bundle for ARM64, run `./build-bundle.sh`. It writes `dist/HxTermux-android-arm64.tar.gz` with both Go binaries and the setup assets. Extract it, enter the `HxTermux` directory, and run `./install.sh`; the bundle can install without Go.

The setup interface lets you select shell tools, dotfiles, Zsh plugins, HypexFetch, Awesomeshot, and an optional Neovim starter. The Zsh experience initializes Oh My Zsh and each configured plugin repository under `~/.oh-my-zsh`, checks that their startup files are present, loads autocomplete before autosuggestions and syntax highlighting, and sets Zsh as the Termux login shell. The appearance studio has Palette, Font, and Prompt tabs; prompt selections update `ZSH_THEME` and persist between runs. The Neovim option installs NvChad's starter, Termux build dependencies, HxTermux editor defaults, and runs the initial plugin sync. Review the selected components and appearance before installation starts. Run `hxtermux` (or `hx`) for the appearance studio, and `hxrestore` or `hxtermux --restore` to restore a saved configuration. Every setup run creates a restore point in `~/.hxtermux-backups`; restoring creates a safety snapshot of the current state first, so you can restore it later. Older single-file backups are also listed and only restore their saved paths. Packages remain installed. Run `hypexfetch` for the system banner.

## Awesomeshot

Awesomeshot is optional. HxTermux installs its official Termux branch and required Termux packages instead of trying to install it as an apt package. Screenshots require the Termux:API app and `termux-api` package.

## Theme and font credits

HxTermux bundles terminal palettes and fonts for local preview and selection. Palette credits:

- [Gogh](https://github.com/Gogh-Co/Gogh): Dracula, Elementary, Flat, Gruvbox, Material, Monokai, One Dark, Snazzy, and Tomorrow Night palettes.
- [Catppuccin for Termux](https://github.com/catppuccin/termux) and [Material Ocean](https://github.com/Material-Ocean).
- [Ayu](https://github.com/ayu-theme/ayu-colors), [Everblush](https://github.com/everblush), [Nekonako](https://github.com/nekonako), [Owl4ce](https://github.com/owl4ce), and [Siduck](https://github.com/siduck) for other bundled palettes. Xshin is credited as the source of the remaining custom palette.

Font credits: [JetBrains Mono](https://github.com/JetBrains/JetBrainsMono), [Fira Code](https://github.com/tonsky/FiraCode), and [Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) for MesloLGS NF. The font files are provided for local selection; retain their upstream licenses when redistributing them.

Awesomeshot is maintained at [Awesomesh0t/awesomeshot](https://github.com/Awesomesh0t/awesomeshot). HypexFetch is the fetch utility included with this project.

## Original project credit

HxTermux builds on the Termux setup and dotfiles work of [Arman Wipangestu](https://github.com/armandwipangestu) and the [myTermux project](https://github.com/mayTermux/myTermux). The repository is maintained as the standalone HxTermux project; the original project's GPL-3.0 license is retained.

## License

See [LICENSE](./LICENSE).
