app [main] {
    pf: platform "https://github.com/roc-lang/basic-cli/releases/download/0.8.1/x8URkvfyi9I0QhmVG98roKBUs_AZRkLFwFJVJ3942YA.tar.br",
    parc: "/Users/pei.ching/privrepo/parc/main.roc",
}

import pf.Stdout
import pf.Stderr
import pf.Task
import pf.Arg
import pf.File
import pf.Path
import parc.Parser
import Cli

main = Cli.main
