
use super::object::Object;
use std::collections::HashMap;

struct Scope<'a>(HashMap<String, Object<'a>>);
pub struct ScopeStack<'a>{
    scopes: Vec<Scope<'a>>,
}

impl Scope<'_> {
    fn new<'a>() -> Scope<'a> {
        Scope(HashMap::new())
    }
}

impl<'scope> ScopeStack<'scope> {
    pub fn new<'a>() -> ScopeStack<'a> {
        ScopeStack { scopes: vec![Scope::new()] }
    }

    pub fn find<'this, 'obj: 'this>(&'this self, name: &str) -> Option<Object<'obj>> {
        self
            .scopes
            .iter()
            .rev()
            .find_map(|scope| scope.0.get(name).map(|obj| obj.clone()))
    }

    pub fn declare(&mut self, name: String, val: Object) {
        self
            .scopes
            .last_mut()
            .expect("bug: empty scope stack")
            .0
            .insert(name, val);
    }

    pub fn add_scope(&mut self) {
        self.scopes.push(Scope::new())
    }
}
