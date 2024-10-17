
import { file_auth_v1_auth } from './auth/v1/auth_pb.ts';
import { file_collection_v1_collection } from './collection/v1/collection_pb.ts';
import { file_environment_v1_environment } from './environment/v1/environment_pb.ts';
import { file_variable_v1_variable } from './variable/v1/variable_pb.ts';
import { file_workspace_v1_workspace } from './workspace/v1/workspace_pb.ts';
import { file_collection_item_v1_item } from './collection/item/v1/item_pb.ts';
import { file_collection_item_body_v1_body } from './collection/item/body/v1/body_pb.ts';
import { file_collection_item_endpoint_v1_endpoint } from './collection/item/endpoint/v1/endpoint_pb.ts';
import { file_collection_item_example_v1_example } from './collection/item/example/v1/example_pb.ts';
import { file_collection_item_folder_v1_folder } from './collection/item/folder/v1/folder_pb.ts';
import { file_collection_item_request_v1_request } from './collection/item/request/v1/request_pb.ts';
import { file_collection_item_response_v1_response } from './collection/item/response/v1/response_pb.ts';

export const files = [
  file_auth_v1_auth,
  file_collection_v1_collection,
  file_environment_v1_environment,
  file_variable_v1_variable,
  file_workspace_v1_workspace,
  file_collection_item_v1_item,
  file_collection_item_body_v1_body,
  file_collection_item_endpoint_v1_endpoint,
  file_collection_item_example_v1_example,
  file_collection_item_folder_v1_folder,
  file_collection_item_request_v1_request,
  file_collection_item_response_v1_response,
];
