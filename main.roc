app "typee"
    packages {
        pf: "https://github.com/roc-lang/basic-cli/releases/download/0.8.1/x8URkvfyi9I0QhmVG98roKBUs_AZRkLFwFJVJ3942YA.tar.br",
        parc: "/home/nixos/repo/parc/main.roc",
    }
    imports [
        pf.Task,
        pf.Arg,
        pf.File,
        pf.Path,
        parc.Parser,
        Lex,
        Parse,
        Backend.StackVm.Machine,
        Cli,
    ]
    provides [main] to pf

main = Cli.main
