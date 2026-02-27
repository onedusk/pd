class User:
    """Represents a system user."""

    def __init__(self, name: str, email: str) -> None:
        self.name = name
        self.email = email
        self.id: int | None = None

    def is_valid(self) -> bool:
        return "@" in self.email


def _generate_id() -> int:
    """Private helper to generate user IDs."""
    import random
    return random.randint(1, 10000)


def create_user(name: str, email: str) -> User:
    user = User(name, email)
    user.id = _generate_id()
    return user
