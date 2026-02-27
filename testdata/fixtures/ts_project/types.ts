export interface User {
  id: number;
  name: string;
  email: string;
}

export type UserRole = "admin" | "member" | "guest";

export enum Status {
  Active = "active",
  Inactive = "inactive",
}

function validateEmail(email: string): boolean {
  return email.includes("@");
}

export default validateEmail;
