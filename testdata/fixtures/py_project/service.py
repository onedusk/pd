from .models import User, create_user


class UserService:
    """Handles user business logic."""

    def __init__(self) -> None:
        self._users: list[User] = []

    def get_user(self, user_id: int) -> User | None:
        for user in self._users:
            if user.id == user_id:
                return user
        return None

    def create(self, name: str, email: str) -> User:
        user = create_user(name, email)
        self._users.append(user)
        return user
