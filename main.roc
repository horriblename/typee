app "typee"
    packages {
        pf: "https://github.com/roc-lang/basic-cli/releases/download/0.8.1/x8URkvfyi9I0QhmVG98roKBUs_AZRkLFwFJVJ3942YA.tar.br",
        parc: "/home/py/repo/parc/Parc/main.roc",
    }
    imports [ pf.Stdout, Lex, parc.Parser, parc.Combinator.{alt}]
    provides [main] to pf

# parser = tag "hi" |> andThen (tag "hi")

expect Lex.lex "()" == Ok [LParen, RParen]

main = Stdout.line "hello"
