import { UserService } from "./service";
import validateEmail from "./types";

const service = new UserService();

export function main(): void {
  if (validateEmail("test@example.com")) {
    service.create("Test", "test@example.com");
  }
}
