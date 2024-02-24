app "typee"
    packages {
        pf: "https://github.com/roc-lang/basic-cli/releases/download/0.8.1/x8URkvfyi9I0QhmVG98roKBUs_AZRkLFwFJVJ3942YA.tar.br",
        parc: "/home/nixos/repo/parc/main.roc",
    }
    imports [ pf.Stdout, parc.Parser, Lex, Parse]
    provides [main] to pf

main = Stdout.line "hello"
