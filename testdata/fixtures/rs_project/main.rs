mod model;
mod service;

use model::User;

fn main() {
    let user = User::new("Alice".to_string(), "alice@example.com".to_string());
    println!("Created user: {}", user.name);
}
