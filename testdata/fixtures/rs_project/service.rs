use crate::model::{Repository, User};

pub struct UserService<R: Repository> {
    repo: R,
}

impl<R: Repository> UserService<R> {
    pub fn new(repo: R) -> Self {
        UserService { repo }
    }

    pub fn get_user(&self, id: u64) -> Option<&User> {
        self.repo.find_by_id(id)
    }

    pub fn create_user(&mut self, name: String, email: String) -> Result<(), String> {
        let user = User::new(name, email);
        self.repo.save(user)
    }
}
