import { version } from "antd";

// For more information about MLflow data structures, see https://mlflow.org/docs/latest/rest-api.html.

interface RegisteredModel {
  name: string;
  created_timestamp: Number;
  last_updated_timestamp: number;
  user_id?: string;
  description?: string;
  latest_versions?: Array<ModelVersion>;
  tags?: Array<RegisteredModelTag>;
  aliases?: Array<RegisteredModelAlias>;
}

interface RegisteredModelTag {
  key: string;
  value: string;
}

interface RegisteredModelAlias {
  alias: string;
  version: string;
}

interface ModelVersion {
  name: string;
  version: string;
  creation_timestamp?: number;
  last_updated_timestamp?: number;
  user_id?: string;
  current_stage?: string;
  description?: string;
  source?: string;
  run_id?: string;
  status?: ModelVersionStatus;
  status_message?: string;
  tags?: Array<ModelVersionTag>;
  run_link?: string;
  aliases?: Array<string>;
}

enum ModelVersionStatus {
  PENDING_REGISTRATION,
  FAILED_REGISTRATION,
  READY,
}

interface ModelVersionTag {
  key: string;
  value: string;
}
