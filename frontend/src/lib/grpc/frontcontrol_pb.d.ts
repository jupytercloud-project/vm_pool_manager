import * as jspb from 'google-protobuf'

import * as google_protobuf_empty_pb from 'google-protobuf/google/protobuf/empty_pb'; // proto import: "google/protobuf/empty.proto"
import * as google_protobuf_timestamp_pb from 'google-protobuf/google/protobuf/timestamp_pb'; // proto import: "google/protobuf/timestamp.proto"


export class CreateUserRequest extends jspb.Message {
  getUsername(): string;
  setUsername(value: string): CreateUserRequest;

  getPassword(): string;
  setPassword(value: string): CreateUserRequest;

  getEmail(): string;
  setEmail(value: string): CreateUserRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateUserRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreateUserRequest): CreateUserRequest.AsObject;
  static serializeBinaryToWriter(message: CreateUserRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateUserRequest;
  static deserializeBinaryFromReader(message: CreateUserRequest, reader: jspb.BinaryReader): CreateUserRequest;
}

export namespace CreateUserRequest {
  export type AsObject = {
    username: string;
    password: string;
    email: string;
  };
}

export class CreateUserResponse extends jspb.Message {
  getUserId(): string;
  setUserId(value: string): CreateUserResponse;

  getSuccess(): boolean;
  setSuccess(value: boolean): CreateUserResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateUserResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CreateUserResponse): CreateUserResponse.AsObject;
  static serializeBinaryToWriter(message: CreateUserResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateUserResponse;
  static deserializeBinaryFromReader(message: CreateUserResponse, reader: jspb.BinaryReader): CreateUserResponse;
}

export namespace CreateUserResponse {
  export type AsObject = {
    userId: string;
    success: boolean;
  };
}

export class AuthenticateUserRequest extends jspb.Message {
  getEmail(): string;
  setEmail(value: string): AuthenticateUserRequest;

  getPassword(): string;
  setPassword(value: string): AuthenticateUserRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AuthenticateUserRequest.AsObject;
  static toObject(includeInstance: boolean, msg: AuthenticateUserRequest): AuthenticateUserRequest.AsObject;
  static serializeBinaryToWriter(message: AuthenticateUserRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AuthenticateUserRequest;
  static deserializeBinaryFromReader(message: AuthenticateUserRequest, reader: jspb.BinaryReader): AuthenticateUserRequest;
}

export namespace AuthenticateUserRequest {
  export type AsObject = {
    email: string;
    password: string;
  };
}

export class AuthenticateUserResponse extends jspb.Message {
  getToken(): string;
  setToken(value: string): AuthenticateUserResponse;

  getSuccess(): boolean;
  setSuccess(value: boolean): AuthenticateUserResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AuthenticateUserResponse.AsObject;
  static toObject(includeInstance: boolean, msg: AuthenticateUserResponse): AuthenticateUserResponse.AsObject;
  static serializeBinaryToWriter(message: AuthenticateUserResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AuthenticateUserResponse;
  static deserializeBinaryFromReader(message: AuthenticateUserResponse, reader: jspb.BinaryReader): AuthenticateUserResponse;
}

export namespace AuthenticateUserResponse {
  export type AsObject = {
    token: string;
    success: boolean;
  };
}

export class Image extends jspb.Message {
  getId(): string;
  setId(value: string): Image;

  getName(): string;
  setName(value: string): Image;

  getStatus(): string;
  setStatus(value: string): Image;

  getTags(): string;
  setTags(value: string): Image;

  getContainerFormat(): string;
  setContainerFormat(value: string): Image;

  getDiskFormat(): string;
  setDiskFormat(value: string): Image;

  getMinDiskGigabytes(): number;
  setMinDiskGigabytes(value: number): Image;

  getMinRamMegabytes(): number;
  setMinRamMegabytes(value: number): Image;

  getOwner(): string;
  setOwner(value: string): Image;

  getProtected(): boolean;
  setProtected(value: boolean): Image;

  getVisibility(): string;
  setVisibility(value: string): Image;

  getHidden(): boolean;
  setHidden(value: boolean): Image;

  getChecksum(): string;
  setChecksum(value: string): Image;

  getSizeBytes(): number;
  setSizeBytes(value: number): Image;

  getCreatedAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setCreatedAt(value?: google_protobuf_timestamp_pb.Timestamp): Image;
  hasCreatedAt(): boolean;
  clearCreatedAt(): Image;

  getUpdatedAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setUpdatedAt(value?: google_protobuf_timestamp_pb.Timestamp): Image;
  hasUpdatedAt(): boolean;
  clearUpdatedAt(): Image;

  getFile(): string;
  setFile(value: string): Image;

  getSchema(): string;
  setSchema(value: string): Image;

  getVirtualSize(): number;
  setVirtualSize(value: number): Image;

  getImportMethods(): string;
  setImportMethods(value: string): Image;

  getStoreIds(): string;
  setStoreIds(value: string): Image;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Image.AsObject;
  static toObject(includeInstance: boolean, msg: Image): Image.AsObject;
  static serializeBinaryToWriter(message: Image, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Image;
  static deserializeBinaryFromReader(message: Image, reader: jspb.BinaryReader): Image;
}

export namespace Image {
  export type AsObject = {
    id: string;
    name: string;
    status: string;
    tags: string;
    containerFormat: string;
    diskFormat: string;
    minDiskGigabytes: number;
    minRamMegabytes: number;
    owner: string;
    pb_protected: boolean;
    visibility: string;
    hidden: boolean;
    checksum: string;
    sizeBytes: number;
    createdAt?: google_protobuf_timestamp_pb.Timestamp.AsObject;
    updatedAt?: google_protobuf_timestamp_pb.Timestamp.AsObject;
    file: string;
    schema: string;
    virtualSize: number;
    importMethods: string;
    storeIds: string;
  };
}

export class Flavor extends jspb.Message {
  getId(): string;
  setId(value: string): Flavor;

  getName(): string;
  setName(value: string): Flavor;

  getDisk(): number;
  setDisk(value: number): Flavor;

  getRam(): number;
  setRam(value: number): Flavor;

  getVcpus(): number;
  setVcpus(value: number): Flavor;

  getRxtxFactor(): number;
  setRxtxFactor(value: number): Flavor;

  getSwap(): number;
  setSwap(value: number): Flavor;

  getEphemeral(): number;
  setEphemeral(value: number): Flavor;

  getIsPublic(): boolean;
  setIsPublic(value: boolean): Flavor;

  getDescription(): string;
  setDescription(value: string): Flavor;

  getExtraSpecs(): string;
  setExtraSpecs(value: string): Flavor;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Flavor.AsObject;
  static toObject(includeInstance: boolean, msg: Flavor): Flavor.AsObject;
  static serializeBinaryToWriter(message: Flavor, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Flavor;
  static deserializeBinaryFromReader(message: Flavor, reader: jspb.BinaryReader): Flavor;
}

export namespace Flavor {
  export type AsObject = {
    id: string;
    name: string;
    disk: number;
    ram: number;
    vcpus: number;
    rxtxFactor: number;
    swap: number;
    ephemeral: number;
    isPublic: boolean;
    description: string;
    extraSpecs: string;
  };
}

export class Network extends jspb.Message {
  getId(): string;
  setId(value: string): Network;

  getName(): string;
  setName(value: string): Network;

  getDescription(): string;
  setDescription(value: string): Network;

  getAdminStateUp(): boolean;
  setAdminStateUp(value: boolean): Network;

  getStatus(): string;
  setStatus(value: string): Network;

  getTenantId(): string;
  setTenantId(value: string): Network;

  getProjectId(): string;
  setProjectId(value: string): Network;

  getShared(): boolean;
  setShared(value: boolean): Network;

  getRevisionNumber(): number;
  setRevisionNumber(value: number): Network;

  getSubnets(): string;
  setSubnets(value: string): Network;

  getAvailabilityZoneHints(): string;
  setAvailabilityZoneHints(value: string): Network;

  getTags(): string;
  setTags(value: string): Network;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Network.AsObject;
  static toObject(includeInstance: boolean, msg: Network): Network.AsObject;
  static serializeBinaryToWriter(message: Network, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Network;
  static deserializeBinaryFromReader(message: Network, reader: jspb.BinaryReader): Network;
}

export namespace Network {
  export type AsObject = {
    id: string;
    name: string;
    description: string;
    adminStateUp: boolean;
    status: string;
    tenantId: string;
    projectId: string;
    shared: boolean;
    revisionNumber: number;
    subnets: string;
    availabilityZoneHints: string;
    tags: string;
  };
}

export class CreateConfigRequest extends jspb.Message {
  getUser(): string;
  setUser(value: string): CreateConfigRequest;

  getKey(): string;
  setKey(value: string): CreateConfigRequest;

  getValue(): string;
  setValue(value: string): CreateConfigRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreateConfigRequest): CreateConfigRequest.AsObject;
  static serializeBinaryToWriter(message: CreateConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateConfigRequest;
  static deserializeBinaryFromReader(message: CreateConfigRequest, reader: jspb.BinaryReader): CreateConfigRequest;
}

export namespace CreateConfigRequest {
  export type AsObject = {
    user: string;
    key: string;
    value: string;
  };
}

export class CreateConfigResponse extends jspb.Message {
  getSuccess(): boolean;
  setSuccess(value: boolean): CreateConfigResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateConfigResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CreateConfigResponse): CreateConfigResponse.AsObject;
  static serializeBinaryToWriter(message: CreateConfigResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateConfigResponse;
  static deserializeBinaryFromReader(message: CreateConfigResponse, reader: jspb.BinaryReader): CreateConfigResponse;
}

export namespace CreateConfigResponse {
  export type AsObject = {
    success: boolean;
  };
}

export class UpdateConfigRequest extends jspb.Message {
  getUser(): string;
  setUser(value: string): UpdateConfigRequest;

  getKey(): string;
  setKey(value: string): UpdateConfigRequest;

  getValue(): string;
  setValue(value: string): UpdateConfigRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: UpdateConfigRequest): UpdateConfigRequest.AsObject;
  static serializeBinaryToWriter(message: UpdateConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdateConfigRequest;
  static deserializeBinaryFromReader(message: UpdateConfigRequest, reader: jspb.BinaryReader): UpdateConfigRequest;
}

export namespace UpdateConfigRequest {
  export type AsObject = {
    user: string;
    key: string;
    value: string;
  };
}

export class UpdateConfigResponse extends jspb.Message {
  getSuccess(): boolean;
  setSuccess(value: boolean): UpdateConfigResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateConfigResponse.AsObject;
  static toObject(includeInstance: boolean, msg: UpdateConfigResponse): UpdateConfigResponse.AsObject;
  static serializeBinaryToWriter(message: UpdateConfigResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdateConfigResponse;
  static deserializeBinaryFromReader(message: UpdateConfigResponse, reader: jspb.BinaryReader): UpdateConfigResponse;
}

export namespace UpdateConfigResponse {
  export type AsObject = {
    success: boolean;
  };
}

export class DeleteConfigRequest extends jspb.Message {
  getUser(): string;
  setUser(value: string): DeleteConfigRequest;

  getKey(): string;
  setKey(value: string): DeleteConfigRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DeleteConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: DeleteConfigRequest): DeleteConfigRequest.AsObject;
  static serializeBinaryToWriter(message: DeleteConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DeleteConfigRequest;
  static deserializeBinaryFromReader(message: DeleteConfigRequest, reader: jspb.BinaryReader): DeleteConfigRequest;
}

export namespace DeleteConfigRequest {
  export type AsObject = {
    user: string;
    key: string;
  };
}

export class DeleteConfigResponse extends jspb.Message {
  getSuccess(): boolean;
  setSuccess(value: boolean): DeleteConfigResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DeleteConfigResponse.AsObject;
  static toObject(includeInstance: boolean, msg: DeleteConfigResponse): DeleteConfigResponse.AsObject;
  static serializeBinaryToWriter(message: DeleteConfigResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DeleteConfigResponse;
  static deserializeBinaryFromReader(message: DeleteConfigResponse, reader: jspb.BinaryReader): DeleteConfigResponse;
}

export namespace DeleteConfigResponse {
  export type AsObject = {
    success: boolean;
  };
}

export class GetConfigRequest extends jspb.Message {
  getUser(): string;
  setUser(value: string): GetConfigRequest;

  getKey(): string;
  setKey(value: string): GetConfigRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetConfigRequest): GetConfigRequest.AsObject;
  static serializeBinaryToWriter(message: GetConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetConfigRequest;
  static deserializeBinaryFromReader(message: GetConfigRequest, reader: jspb.BinaryReader): GetConfigRequest;
}

export namespace GetConfigRequest {
  export type AsObject = {
    user: string;
    key: string;
  };
}

export class GetConfigResponse extends jspb.Message {
  getValue(): string;
  setValue(value: string): GetConfigResponse;

  getKey(): string;
  setKey(value: string): GetConfigResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetConfigResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetConfigResponse): GetConfigResponse.AsObject;
  static serializeBinaryToWriter(message: GetConfigResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetConfigResponse;
  static deserializeBinaryFromReader(message: GetConfigResponse, reader: jspb.BinaryReader): GetConfigResponse;
}

export namespace GetConfigResponse {
  export type AsObject = {
    value: string;
    key: string;
  };
}

export class CreatePoolRequest extends jspb.Message {
  getUser(): string;
  setUser(value: string): CreatePoolRequest;

  getName(): string;
  setName(value: string): CreatePoolRequest;

  getImage(): string;
  setImage(value: string): CreatePoolRequest;

  getFlavor(): string;
  setFlavor(value: string): CreatePoolRequest;

  getNetwork(): string;
  setNetwork(value: string): CreatePoolRequest;

  getConfig(): string;
  setConfig(value: string): CreatePoolRequest;

  getMinVm(): string;
  setMinVm(value: string): CreatePoolRequest;

  getMaxVm(): string;
  setMaxVm(value: string): CreatePoolRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreatePoolRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreatePoolRequest): CreatePoolRequest.AsObject;
  static serializeBinaryToWriter(message: CreatePoolRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreatePoolRequest;
  static deserializeBinaryFromReader(message: CreatePoolRequest, reader: jspb.BinaryReader): CreatePoolRequest;
}

export namespace CreatePoolRequest {
  export type AsObject = {
    user: string;
    name: string;
    image: string;
    flavor: string;
    network: string;
    config: string;
    minVm: string;
    maxVm: string;
  };
}

export class CreatePoolResponse extends jspb.Message {
  getPoolId(): string;
  setPoolId(value: string): CreatePoolResponse;

  getSuccess(): boolean;
  setSuccess(value: boolean): CreatePoolResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreatePoolResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CreatePoolResponse): CreatePoolResponse.AsObject;
  static serializeBinaryToWriter(message: CreatePoolResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreatePoolResponse;
  static deserializeBinaryFromReader(message: CreatePoolResponse, reader: jspb.BinaryReader): CreatePoolResponse;
}

export namespace CreatePoolResponse {
  export type AsObject = {
    poolId: string;
    success: boolean;
  };
}

export class DeletePoolRequest extends jspb.Message {
  getUser(): string;
  setUser(value: string): DeletePoolRequest;

  getPoolId(): string;
  setPoolId(value: string): DeletePoolRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DeletePoolRequest.AsObject;
  static toObject(includeInstance: boolean, msg: DeletePoolRequest): DeletePoolRequest.AsObject;
  static serializeBinaryToWriter(message: DeletePoolRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DeletePoolRequest;
  static deserializeBinaryFromReader(message: DeletePoolRequest, reader: jspb.BinaryReader): DeletePoolRequest;
}

export namespace DeletePoolRequest {
  export type AsObject = {
    user: string;
    poolId: string;
  };
}

export class DeletePoolResponse extends jspb.Message {
  getSuccess(): boolean;
  setSuccess(value: boolean): DeletePoolResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DeletePoolResponse.AsObject;
  static toObject(includeInstance: boolean, msg: DeletePoolResponse): DeletePoolResponse.AsObject;
  static serializeBinaryToWriter(message: DeletePoolResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DeletePoolResponse;
  static deserializeBinaryFromReader(message: DeletePoolResponse, reader: jspb.BinaryReader): DeletePoolResponse;
}

export namespace DeletePoolResponse {
  export type AsObject = {
    success: boolean;
  };
}

export class GetPoolRequest extends jspb.Message {
  getUser(): string;
  setUser(value: string): GetPoolRequest;

  getPoolId(): string;
  setPoolId(value: string): GetPoolRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetPoolRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetPoolRequest): GetPoolRequest.AsObject;
  static serializeBinaryToWriter(message: GetPoolRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetPoolRequest;
  static deserializeBinaryFromReader(message: GetPoolRequest, reader: jspb.BinaryReader): GetPoolRequest;
}

export namespace GetPoolRequest {
  export type AsObject = {
    user: string;
    poolId: string;
  };
}

export class GetPoolResponse extends jspb.Message {
  getName(): string;
  setName(value: string): GetPoolResponse;

  getImage(): string;
  setImage(value: string): GetPoolResponse;

  getFlavor(): string;
  setFlavor(value: string): GetPoolResponse;

  getNetwork(): string;
  setNetwork(value: string): GetPoolResponse;

  getConfig(): string;
  setConfig(value: string): GetPoolResponse;

  getMinVm(): number;
  setMinVm(value: number): GetPoolResponse;

  getMaxVm(): number;
  setMaxVm(value: number): GetPoolResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetPoolResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetPoolResponse): GetPoolResponse.AsObject;
  static serializeBinaryToWriter(message: GetPoolResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetPoolResponse;
  static deserializeBinaryFromReader(message: GetPoolResponse, reader: jspb.BinaryReader): GetPoolResponse;
}

export namespace GetPoolResponse {
  export type AsObject = {
    name: string;
    image: string;
    flavor: string;
    network: string;
    config: string;
    minVm: number;
    maxVm: number;
  };
}

export class UpdateDataUserRequest extends jspb.Message {
  getUser(): string;
  setUser(value: string): UpdateDataUserRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateDataUserRequest.AsObject;
  static toObject(includeInstance: boolean, msg: UpdateDataUserRequest): UpdateDataUserRequest.AsObject;
  static serializeBinaryToWriter(message: UpdateDataUserRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdateDataUserRequest;
  static deserializeBinaryFromReader(message: UpdateDataUserRequest, reader: jspb.BinaryReader): UpdateDataUserRequest;
}

export namespace UpdateDataUserRequest {
  export type AsObject = {
    user: string;
  };
}

export class UpdateDataUserResponse extends jspb.Message {
  getUser(): string;
  setUser(value: string): UpdateDataUserResponse;

  getStatus(): Status;
  setStatus(value: Status): UpdateDataUserResponse;

  getType(): Type;
  setType(value: Type): UpdateDataUserResponse;

  getDataMap(): jspb.Map<string, string>;
  clearDataMap(): UpdateDataUserResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateDataUserResponse.AsObject;
  static toObject(includeInstance: boolean, msg: UpdateDataUserResponse): UpdateDataUserResponse.AsObject;
  static serializeBinaryToWriter(message: UpdateDataUserResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdateDataUserResponse;
  static deserializeBinaryFromReader(message: UpdateDataUserResponse, reader: jspb.BinaryReader): UpdateDataUserResponse;
}

export namespace UpdateDataUserResponse {
  export type AsObject = {
    user: string;
    status: Status;
    type: Type;
    dataMap: Array<[string, string]>;
  };
}

export class RebuildServerRequest extends jspb.Message {
  getUser(): string;
  setUser(value: string): RebuildServerRequest;

  getPoolId(): string;
  setPoolId(value: string): RebuildServerRequest;

  getServerId(): string;
  setServerId(value: string): RebuildServerRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RebuildServerRequest.AsObject;
  static toObject(includeInstance: boolean, msg: RebuildServerRequest): RebuildServerRequest.AsObject;
  static serializeBinaryToWriter(message: RebuildServerRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RebuildServerRequest;
  static deserializeBinaryFromReader(message: RebuildServerRequest, reader: jspb.BinaryReader): RebuildServerRequest;
}

export namespace RebuildServerRequest {
  export type AsObject = {
    user: string;
    poolId: string;
    serverId: string;
  };
}

export class RebuildServerResponse extends jspb.Message {
  getSuccess(): boolean;
  setSuccess(value: boolean): RebuildServerResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RebuildServerResponse.AsObject;
  static toObject(includeInstance: boolean, msg: RebuildServerResponse): RebuildServerResponse.AsObject;
  static serializeBinaryToWriter(message: RebuildServerResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RebuildServerResponse;
  static deserializeBinaryFromReader(message: RebuildServerResponse, reader: jspb.BinaryReader): RebuildServerResponse;
}

export namespace RebuildServerResponse {
  export type AsObject = {
    success: boolean;
  };
}

export class UserRequest extends jspb.Message {
  getUser(): string;
  setUser(value: string): UserRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UserRequest.AsObject;
  static toObject(includeInstance: boolean, msg: UserRequest): UserRequest.AsObject;
  static serializeBinaryToWriter(message: UserRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UserRequest;
  static deserializeBinaryFromReader(message: UserRequest, reader: jspb.BinaryReader): UserRequest;
}

export namespace UserRequest {
  export type AsObject = {
    user: string;
  };
}

export class Server extends jspb.Message {
  getId(): string;
  setId(value: string): Server;

  getName(): string;
  setName(value: string): Server;

  getStatus(): string;
  setStatus(value: string): Server;

  getImage(): string;
  setImage(value: string): Server;

  getFlavor(): string;
  setFlavor(value: string): Server;

  getNetwork(): string;
  setNetwork(value: string): Server;

  getIpAddress(): string;
  setIpAddress(value: string): Server;

  getCreatedAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setCreatedAt(value?: google_protobuf_timestamp_pb.Timestamp): Server;
  hasCreatedAt(): boolean;
  clearCreatedAt(): Server;

  getUpdatedAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setUpdatedAt(value?: google_protobuf_timestamp_pb.Timestamp): Server;
  hasUpdatedAt(): boolean;
  clearUpdatedAt(): Server;

  getMetadataMap(): jspb.Map<string, string>;
  clearMetadataMap(): Server;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Server.AsObject;
  static toObject(includeInstance: boolean, msg: Server): Server.AsObject;
  static serializeBinaryToWriter(message: Server, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Server;
  static deserializeBinaryFromReader(message: Server, reader: jspb.BinaryReader): Server;
}

export namespace Server {
  export type AsObject = {
    id: string;
    name: string;
    status: string;
    image: string;
    flavor: string;
    network: string;
    ipAddress: string;
    createdAt?: google_protobuf_timestamp_pb.Timestamp.AsObject;
    updatedAt?: google_protobuf_timestamp_pb.Timestamp.AsObject;
    metadataMap: Array<[string, string]>;
  };
}

export class ServerPool extends jspb.Message {
  getId(): string;
  setId(value: string): ServerPool;

  getName(): string;
  setName(value: string): ServerPool;

  getImage(): string;
  setImage(value: string): ServerPool;

  getFlavor(): string;
  setFlavor(value: string): ServerPool;

  getNetwork(): string;
  setNetwork(value: string): ServerPool;

  getConfig(): string;
  setConfig(value: string): ServerPool;

  getMinVm(): number;
  setMinVm(value: number): ServerPool;

  getMaxVm(): number;
  setMaxVm(value: number): ServerPool;

  getMetadataMap(): jspb.Map<string, string>;
  clearMetadataMap(): ServerPool;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServerPool.AsObject;
  static toObject(includeInstance: boolean, msg: ServerPool): ServerPool.AsObject;
  static serializeBinaryToWriter(message: ServerPool, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServerPool;
  static deserializeBinaryFromReader(message: ServerPool, reader: jspb.BinaryReader): ServerPool;
}

export namespace ServerPool {
  export type AsObject = {
    id: string;
    name: string;
    image: string;
    flavor: string;
    network: string;
    config: string;
    minVm: number;
    maxVm: number;
    metadataMap: Array<[string, string]>;
  };
}

export enum Status {
  STATUS_UNKNOWN = 0,
  CREATE = 1,
  UPDATE = 2,
  DELETE = 3,
}
export enum Type {
  TYPE_UNKNOWN = 0,
  SERVERPOOL = 1,
  SERVER = 2,
  CONFIG = 3,
}
