import request from "@/utils/request";
import { ModelVersionTag, RegisteredModel, ModelVersion } from "./types";

// For more information, please visit https://mlflow.org/docs/latest/rest-api.html.
export class ModelRegistryService {
  static checkHealth = () => {
    return fetch("/mlflow/health", {
      method: "GET",
    });
  };

  static createRegisteredModel = (
    name: string,
    tags?: ModelVersionTag[],
    description?: string
  ) => {
    return request<{ registered_model: RegisteredModel }>(
      "/mlflow/api/2.0/mlflow/registered-models/create",
      {
        method: "POST",
        data: {
          name,
          tags,
          description,
        },
      }
    );
  };

  static getRegisteredModel = (name: string) => {
    return request<{ registered_model: RegisteredModel }>(
      "/mlflow/api/2.0/mlflow/registered-models/get",
      {
        method: "GET",
        params: {
          name,
        },
      }
    );
  };

  static searchRegisteredModels = (
    filter: string,
    maxResults?: number,
    orderBy?: string,
    pageToken?: string
  ) => {
    return request<{
      registered_models: RegisteredModel[];
      next_page_token: string;
    }>("/mlflow/api/2.0/mlflow/registered-models/search", {
      method: "GET",
      params: {
        filter,
        max_results: maxResults,
        order_by: orderBy,
        page_token: pageToken,
      },
    });
  };

  static renameRegisteredModel = (name: string, newName: string) => {
    return request<{ registered_model: RegisteredModel }>(
      "/mlflow/api/2.0/mlflow/registered-models/rename",
      {
        method: "POST",
        data: {
          name,
          new_name: newName,
        },
      }
    );
  };

  static updateRegisteredModel = (name: string, description: string) => {
    return request<{ registered_model: RegisteredModel }>(
      "/mlflow/api/2.0/mlflow/registered-models/update",
      {
        method: "PATCH",
        data: {
          name,
          description,
        },
      }
    );
  };

  static setRegisteredModelTag = (
    name: string,
    key: string,
    value?: string
  ) => {
    return request("/mlflow/api/2.0/mlflow/registered-models/set-tag", {
      method: "POST",
      data: {
        name,
        key,
        value: value,
      },
    });
  };

  static setRegisteredModelAlias = (
    name: string,
    alias: string,
    version: string
  ) => {
    return request("/mlflow/api/2.0/mlflow/registered-models/set-alias", {
      method: "POST",
      data: {
        name,
        alias,
        version,
      },
    });
  };

  static deleteRegisteredModel = (name: string) => {
    return request("/mlflow/api/2.0/mlflow/registered-models/delete", {
      method: "DELETE",
      data: {
        name,
      },
    });
  };

  static deleteRegisteredModelTag = (name: string, key: string) => {
    return request("/mlflow/api/2.0/mlflow/registered-models/delete-tag", {
      method: "DELETE",
      data: {
        name,
        key,
      },
    });
  };

  static deleteRegisteredModelAlias = (name: string, alias: string) => {
    return request("/mlflow/api/2.0/mlflow/registered-models/delete-alias", {
      method: "DELETE",
      data: {
        name,
        alias,
      },
    });
  };

  static getLatestModelVersions = (name: string, stages: string[]) => {
    return request<{ modelVersions: ModelVersion[] }>(
      "/mlflow/api/2.0/mlflow/registered-models/get-latest-versions",
      {
        method: "POST",
        data: {
          name,
          stages,
        },
      }
    );
  };

  static createModelVersion = (
    name: string,
    source?: string,
    runId?: string,
    tags?: ModelVersionTag[],
    runLink?: string,
    description?: string
  ) => {
    return request<{ model_version: ModelVersion }>(
      "/mlflow/api/2.0/mlflow/model-versions/create",
      {
        method: "POST",
        data: {
          name,
          source,
          run_id: runId,
          tags,
          run_link: runLink,
          description,
        },
      }
    );
  };

  static getModelVersion = (name: string, version: string) => {
    return request<{ model_version: ModelVersion }>(
      "/mlflow/api/2.0/mlflow/model-versions/get",
      {
        method: "GET",
        params: {
          name,
          version,
        },
      }
    );
  };

  static getModelVersionByAlias = (name: string, alias: string) => {
    return request<{ model_version: ModelVersion }>(
      "/mlflow/api/2.0/mlflow/model-versions/alias",
      {
        method: "GET",
        params: {
          name,
          alias,
        },
      }
    );
  };

  static searchModelVersions = (
    filter: string,
    maxResults?: number,
    orderBy?: string[],
    pageToken?: string
  ) => {
    return request<{ model_versions: ModelVersion[]; next_page_token: string }>(
      "/mlflow/api/2.0/mlflow/model-versions/search",
      {
        method: "GET",
        params: {
          filter,
          max_results: maxResults,
          order_by: orderBy,
          page_token: pageToken,
        },
      }
    );
  };

  static updateModelVersion = (
    name: string,
    version: string,
    description: string
  ) => {
    return request<{ model_version: ModelVersion }>(
      "/mlflow/api/2.0/mlflow/model-versions/update",
      {
        method: "PATCH",
        data: {
          name,
          version,
          description,
        },
      }
    );
  };

  static setModelVersionTag = (
    name: string,
    version: string,
    key: string,
    value?: string
  ) => {
    return request("/mlflow/api/2.0/mlflow/model-versions/set-tag", {
      method: "POST",
      data: {
        name,
        version,
        key,
        value,
      },
    });
  };

  static deleteModelVersion = (name: string, version: string) => {
    return request("/mlflow/api/2.0/mlflow/model-versions/delete", {
      method: "DELETE",
      data: {
        name,
        version,
      },
    });
  };

  static deleteModelVersionTag = (
    name: string,
    version: string,
    key: string
  ) => {
    return request("/mlflow/api/2.0/mlflow/model-versions/delete-tag", {
      method: "DELETE",
      data: {
        name,
        version,
        key,
      },
    });
  };

  // Get download URI for model version artifacts
  static getDownloadUri = (name: string, version: string) => {
    return request<{ artifact_uri: string }>(
      "/mlflow/api/2.0/mlflow/model-versions/get-download-uri",
      {
        method: "GET",
        data: {
          name,
          version,
        },
      }
    );
  };

  // Transition model version stage
  static transitionModelVersionStage = (
    name: string,
    version: string,
    stage: string,
    archiveExistingVersions: boolean
  ) => {
    return request<{ model_version: ModelVersion }>(
      "/mlflow/api/2.0/mlflow/model-versions/transition-stage",
      {
        method: "POST",
        data: {
          name,
          version,
          stage,
          archive_existing_versions: archiveExistingVersions,
        },
      }
    );
  };
}
