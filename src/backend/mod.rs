use gccjit::OptimizationLevel;

use crate::ast;
use std::path::PathBuf;

mod object;

struct State<'ctx> {
    ctx: gccjit::Context<'ctx>,
}


pub fn build(program: &ast::Program) {
    let mut state = State::new();
    state.build(program);
    state.finish("b.out".into());
}

impl<'ctx> State<'ctx> {
    fn new() -> Self {
        let ctx =  gccjit::Context::default();
        ctx.set_dump_code_on_compile(true);
        ctx.set_optimization_level(OptimizationLevel::Standard);

        Self {ctx}
    }

    fn build(&mut self, program: &ast::Program) {
        for stmt in program {
            self.build_expr(stmt);
        }
    }

    fn finish(&mut self, filename: PathBuf) -> gccjit::CompileResult {
        self.ctx.compile()
    }

    fn build_expr(&mut self, expr: &ast::Expr) {
        match expr {
            ast::Expr::List(_) => todo!(),
            ast::Expr::Symbol(_) => todo!(),
            ast::Expr::IntLiteral(_) => todo!(),
        }
    }
}

