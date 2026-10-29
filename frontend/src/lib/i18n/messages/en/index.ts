import admin from "./admin.json";
import auth from "./auth.json";
import candidate from "./candidate.json";
import common from "./common.json";
import errors from "./errors.json";
import publicArea from "./public.json";
import recruiter from "./recruiter.json";
import validation from "./validation.json";

// One file per area keeps parallel translation work from touching the same lines.
const messages = {
  common,
  errors,
  validation,
  auth,
  candidate,
  recruiter,
  admin,
  public: publicArea,
};
export default messages;
