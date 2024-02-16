use std::{collections::HashMap, cell::RefCell};
use gccjit::{OptimizationLevel, FunctionType, Type, RValue, ToRValue, LValue, Context, Parameter, Typeable, CompileResult};

use crate::{ast, lex::LexError};

mod object;

pub struct State<'ctx> {
    ctx: gccjit::Context<'ctx>,
    block_stack: Vec<gccjit::Block<'ctx>>,
    scope_stack: RefCell<ScopeStack<'ctx>>,
}

struct ScopeStack<'ctx> {
    stack: Vec<HashMap<String, GccjitObject<'ctx>>>,
}

// could not figure out how to downcast objects, so I'm using this
enum GccjitObject<'ctx> {
    Func(gccjit::Function<'ctx>),
    RValue(RValue<'ctx>),
    LValue(LValue<'ctx>),
    Parameter(Parameter<'ctx>),
}

#[derive(Debug)]
enum CompError {
    InvalidForm,
    InvalidFunctionForm,
    MissingFunctionName,
    MissingFunctionArgs,
    MissingFunctionBody,
    BadFunctionArgDefinition,
    FuncCallWrongArgCount,
    UndefinedSymbol,
    CalledNonFunction,
}

type CompResult< T> = Result<T, CompError>;

struct SymbolExistsError();

impl<'ctx> ScopeStack<'ctx> {
    fn new() -> Self {
        ScopeStack{stack: Vec::new()}
    }

    fn new_scope(&mut self) {
        self.stack.push(HashMap::new())
    }

    fn declare(&mut self, name: String, lval: GccjitObject<'ctx>) -> Result<(), SymbolExistsError> {
        if let Some(_) = self.stack
            .last_mut()
            .expect("compiler BUG: attempted to declare variable when no scope exists!")
            .insert(name, lval) 
        {
            Err(SymbolExistsError())
        } else {
            Ok(())
        }
    }

    fn pop(&mut self) {
        self.stack.pop();
    }

    fn find_var(&self, name: &str) -> Option<&GccjitObject<'ctx>> {
        for scope in self.stack.iter().rev() {
            if let Some(value) = scope.get(name) {
                return Some(value);
            }
        }

        None
    }
}

impl<'ctx> GccjitObject<'ctx> {
    fn to_rvalue(&self) -> Option<RValue<'ctx>> {
        match self {
            GccjitObject::Func(_func) => None,
            GccjitObject::RValue(rval) => Some(rval.to_rvalue()),
            GccjitObject::LValue(lval) => Some(lval.to_rvalue()),
            GccjitObject::Parameter(param) => Some(param.to_rvalue()),
        }
    }
}

pub fn build_to_file(program: &ast::Program, filename: &str) {
    let state = State::new();
    state.build(program).unwrap();
    state.ctx.compile_to_file(gccjit::OutputKind::Executable, filename);
}

pub fn build<'ctx>(program: &ast::Program) -> i32 {
    let state = State::new();
    state.build(program).unwrap();
    let code = state.ctx.compile();
    let main_func = code.get_function("main");

    let main_func: extern "C" fn() -> i32 = if !main_func.is_null() {
        unsafe {std::mem::transmute(main_func)}
    } else {
        panic!("could not retrieve function")
    };

    main_func()
}

fn nil_ty<'ctx>(ctx: &'ctx Context) -> gccjit::Type<'ctx> {
    ctx.new_type::<()>()
}

fn int_ty<'ctx>(ctx: &'ctx Context) -> gccjit::Type<'ctx> {
    ctx.new_type::<i32>()
}


impl<'ctx> State<'ctx> {
    fn new() -> Self {
        let ctx =  gccjit::Context::default();
        ctx.set_dump_code_on_compile(true);
        ctx.set_optimization_level(OptimizationLevel::Standard);

         Self {ctx, block_stack: Vec::new(), scope_stack: RefCell::new(ScopeStack::new())}
    }

    fn nil_ty(&'ctx self) -> gccjit::Type<'ctx> {
        self.ctx.new_type::<()>()
    }

    fn int_ty(&'ctx self) -> gccjit::Type<'ctx> {
        self.ctx.new_type::<i32>()
    }

    fn top_block(&self) -> &gccjit::Block {
        self.block_stack.last().expect("compiler BUG: attempted to access empty block stack!")
    }

    fn build(&'ctx self, program: &ast::Program) -> CompResult<()> {
        // NOTE: currently the "main function" is the entire script
        let main_func = self.ctx.new_function(None, FunctionType::Exported, self.int_ty(), &[], "main", false);
        let body = main_func.new_block("main_function_body");

        for stmt in &program[..program.len() - 1] {
            self.build_expr(stmt);
        }

        let return_val = program
            .last()
            .map(|stmt| self.build_expr(stmt))
            .unwrap_or_else(|| Ok(self.ctx.new_rvalue_zero(self.int_ty())))?;

        body.end_with_return(None, return_val);
        Ok(())
    }

    fn finish(self) -> Self {
        // if block_stack.len() != 0 {
        //     panic!("end of program: stack not empty!");
        // }

        self
    }

    fn build_expr(&'ctx self, expr: &ast::Expr) -> CompResult<RValue> {
        match expr {
            ast::Expr::List(list) => self.build_form(list),
            ast::Expr::Symbol(name) => self
                .scope_stack
                .borrow()
                .find_var(name)
                .and_then(|var| var.to_rvalue())
                .ok_or_else(|| CompError::UndefinedSymbol),
            ast::Expr::IntLiteral(n) => Ok(self.ctx.new_rvalue_from_int(self.int_ty(), *n)),
        }
    }

    fn build_form(&'ctx self, list: &[ast::Expr]) -> CompResult<RValue<'ctx>> {
        let head = list.first().ok_or_else(|| CompError::InvalidForm)?;

        match head {
            ast::Expr::Symbol("def") => self.build_func_def(&list[1..]),
            // ast::Expr::Symbol("set") => self.set_var(&list[1..]),
            ast::Expr::Symbol(func_name) => self.call_func(func_name, &list[1..]),
            _ => Err(CompError::InvalidForm)
        }
    }

    fn build_func_def(&'ctx self, form_args: &[ast::Expr]) -> CompResult<RValue<'ctx>> {
        let Some(ast::Expr::Symbol(name)) = form_args.first() else {
            return Err(CompError::MissingFunctionName);
        };

        let Some(ast::Expr::List(args_list)) = form_args.get(1) else {
            return Err(CompError::MissingFunctionArgs);
        };


        // TODO: multi expr body
        let body = form_args.get(2).ok_or_else(|| CompError::MissingFunctionBody)?;

        let parameters = args_list.iter().map(|arg| self.process_arg(arg)).collect::<Result<Vec<_>, _>>()?;

        let func = self.ctx.new_function(None, FunctionType::Internal, self.nil_ty(), &parameters, name, false);

        let main_block = func.new_block("main_block");

        let ret = self.build_expr(body)?;

        main_block.end_with_return(None, ret);

        // FIXME: return the function
        Ok(self.ctx.new_rvalue_zero(self.nil_ty()))
    }

    fn process_arg(&'ctx self, arg: &ast::Expr) -> CompResult<Parameter<'ctx>> {
        // let Some(ast::Expr::List(params)) = arg else {
        //     return Err(CompError::BadFunctionArg);
        // };
        //
        // if params.len() != 2 {
        //     return Err(CompError::BadFunctionArg);
        // }

        let ast::Expr::Symbol(name) = arg else {
            return Err(CompError::BadFunctionArgDefinition);
        };

        let param = self.ctx.new_parameter(None, self.int_ty(), name.to_string());

        self
            .scope_stack
            .borrow_mut()
            .declare(name.to_string(), GccjitObject::Parameter(param));

        Ok(param)

        // let ast::Expr::Symbol(name) = params.get(1).is_ok_or(|| CompError::BadFunctionArg) else {
        //     return Err(CompError::BadFunctionArg);
        // };
    }

    fn call_func(&'ctx self, name: &str, args: &[ast::Expr]) -> CompResult<RValue> {
        match name {
            "+" => {
                let l = args.get(0).ok_or_else(|| CompError::FuncCallWrongArgCount)?;
                let l = self.build_expr(l)?;
                let r = args.get(1).ok_or_else(|| CompError::FuncCallWrongArgCount)?;
                let r = self.build_expr(r)?;

                Ok(self.ctx.new_binary_op(None, gccjit::BinaryOp::Plus, self.int_ty(), l, r))
            },
            _ => todo!(),
            // _ => self.call_user_func(name, args),
        }
    }

    fn call_user_func(&'ctx self, name: &str, args: &[ast::Expr]) -> CompResult<RValue> {
        let stack = self.scope_stack.borrow();
        let obj = stack.find_var(name).ok_or_else(|| CompError::UndefinedSymbol)?;
        let GccjitObject::Func(func) = obj else {
            return Err(CompError::CalledNonFunction);
        };

        let args_val: Result<Vec<_>, _> = args.iter().map(|arg| self.build_expr(arg)).collect();

        Ok(self.ctx.new_call(None, *func, &args_val?))
    }
}

#[cfg(test)]
mod tests {
    use gccjit::CompileResult;

    use super::build;
    use crate::parse::parse_tokens;

    pub fn compile_from_source<'src>(source: &'src str) ->CompileResult {
        let (_, tokens) = crate::lex::lex(source).unwrap();
        let (_, prog) = parse_tokens(&tokens).unwrap();
        build(&prog).compile()
    }

    #[test]
    fn compile_main() {
        let src = "(+ 2 1)";
        let result = compile_from_source(src);
        let main_func = result.get_function("main");

        let main_func: extern "C" fn() -> i32 = if !main_func.is_null() {
            unsafe {std::mem::transmute(main_func)}
        } else {
            panic!("could not retrieve function")
        };

        assert!(main_func() == 3);
    }
}
