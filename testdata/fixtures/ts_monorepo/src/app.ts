import { log } from "@test/logger";
import { findAll } from "@test/db/queries";
import { helper } from "./utils";

log("starting");
const data = findAll();
helper(data);
