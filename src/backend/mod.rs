use std::collections::HashMap;
use gccjit::{OptimizationLevel, FunctionType, Type, RValue, ToRValue, LValue, Context};

use crate::ast;

mod object;

struct State<'ctx> {
    ctx: gccjit::Context<'ctx>,
    block_stack: Vec<gccjit::Block<'ctx>>,
    scope_stack: ScopeStack<'ctx>,
}

struct ScopeStack<'ctx> {
    stack: Vec<HashMap<String, LValue<'ctx>>>,
}

impl<'ctx> ScopeStack<'ctx> {
    fn new() -> Self {
        ScopeStack{stack: Vec::new()}
    }

    fn new_scope(&mut self) {
        self.stack.push(HashMap::new())
    }

    fn declare(&mut self, name: String, lval: LValue<'ctx>) {
        self.stack
            .last_mut()
            .expect("compiler BUG: attempted to declare variable when no scope exists!")
            .insert(name, lval);
    }
}

pub fn build(program: &ast::Program) {
    let mut state = State::new();
    state.build(program);
    let ctx = state.finish();
    ctx.compile_to_file(gccjit::OutputKind::Executable, "a.out");
}

impl<'ctx> State<'ctx> {
    fn new() -> Self {
        let ctx =  gccjit::Context::default();
        ctx.set_dump_code_on_compile(true);
        ctx.set_optimization_level(OptimizationLevel::Standard);

         Self {ctx, block_stack: Vec::new(), scope_stack: ScopeStack::new()}
    }

    fn nil_ty(&self) -> gccjit::Type {
        self.ctx.new_type::<()>()
    }

    fn int_ty(&self) -> gccjit::Type {
        self.ctx.new_type::<i32>()
    }

    fn top_block(&self) -> &gccjit::Block {
        self.block_stack.last().expect("compiler BUG: attempted to access empty block stack!")
    }

    fn build(&mut self, program: &ast::Program) {
        // NOTE: currently the "main function" is the entire script
        let main_func = self.ctx.new_function(None, FunctionType::Exported, self.nil_ty(), &[], "main", false);

        for stmt in &program[..program.len() - 1] {
            self.build_expr(stmt);
        }
    }

    fn finish(self) -> gccjit::Context<'ctx> {
        // if block_stack.len() != 0 {
        //     panic!("end of program: stack not empty!");
        // }

        self.ctx
    }

    fn build_expr(&mut self, expr: &ast::Expr) -> RValue {
        match expr {
            ast::Expr::List(_) => todo!(),
            ast::Expr::Symbol(_) => todo!(),
            ast::Expr::IntLiteral(n) => self.ctx.new_rvalue_from_int(self.int_ty(), *n),
        }
    }
}

