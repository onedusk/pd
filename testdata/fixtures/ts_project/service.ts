import { User, Status } from "./types";

export class UserService {
  private users: User[] = [];

  findById(id: number): User | undefined {
    return this.users.find((u) => u.id === id);
  }

  create(name: string, email: string): User {
    const user: User = {
      id: this.users.length + 1,
      name,
      email,
    };
    this.users.push(user);
    return user;
  }

  getActiveCount(): number {
    return this.users.length;
  }
}
