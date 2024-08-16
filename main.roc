app "typee"
    packages {
        pf: "https://github.com/roc-lang/basic-cli/releases/download/0.8.1/x8URkvfyi9I0QhmVG98roKBUs_AZRkLFwFJVJ3942YA.tar.br",
        parc: "/home/deck/repo/parc/main.roc",
    }
    imports [
        pf.Stdout,
        pf.Stderr,
        pf.Task,
        pf.Arg,
        pf.File,
        pf.Path,
        parc.Parser,
        Cli,
    ]
    provides [main] to pf

main = Cli.main
