pub struct User {
    pub id: u64,
    pub name: String,
    pub email: String,
}

pub trait Repository {
    fn find_by_id(&self, id: u64) -> Option<&User>;
    fn save(&mut self, user: User) -> Result<(), String>;
}

impl User {
    pub fn new(name: String, email: String) -> Self {
        User { id: 0, name, email }
    }

    fn validate_email(&self) -> bool {
        self.email.contains('@')
    }
}
