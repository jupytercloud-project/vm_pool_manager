import * as jspb from 'google-protobuf'



export class RessourceRequest extends jspb.Message {
  getUserid(): string;
  setUserid(value: string): RessourceRequest;

  getDataMap(): jspb.Map<string, string>;
  clearDataMap(): RessourceRequest;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RessourceRequest.AsObject;
  static toObject(includeInstance: boolean, msg: RessourceRequest): RessourceRequest.AsObject;
  static serializeBinaryToWriter(message: RessourceRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RessourceRequest;
  static deserializeBinaryFromReader(message: RessourceRequest, reader: jspb.BinaryReader): RessourceRequest;
}

export namespace RessourceRequest {
  export type AsObject = {
    userid: string;
    dataMap: Array<[string, string]>;
  };
}

export class RessourceResponse extends jspb.Message {
  getUserid(): string;
  setUserid(value: string): RessourceResponse;

  getDataMap(): jspb.Map<string, string>;
  clearDataMap(): RessourceResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RessourceResponse.AsObject;
  static toObject(includeInstance: boolean, msg: RessourceResponse): RessourceResponse.AsObject;
  static serializeBinaryToWriter(message: RessourceResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RessourceResponse;
  static deserializeBinaryFromReader(message: RessourceResponse, reader: jspb.BinaryReader): RessourceResponse;
}

export namespace RessourceResponse {
  export type AsObject = {
    userid: string;
    dataMap: Array<[string, string]>;
  };
}

export class StreamRessourceResponse extends jspb.Message {
  getUserid(): string;
  setUserid(value: string): StreamRessourceResponse;

  getStatus(): Status;
  setStatus(value: Status): StreamRessourceResponse;

  getDataMap(): jspb.Map<string, string>;
  clearDataMap(): StreamRessourceResponse;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): StreamRessourceResponse.AsObject;
  static toObject(includeInstance: boolean, msg: StreamRessourceResponse): StreamRessourceResponse.AsObject;
  static serializeBinaryToWriter(message: StreamRessourceResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): StreamRessourceResponse;
  static deserializeBinaryFromReader(message: StreamRessourceResponse, reader: jspb.BinaryReader): StreamRessourceResponse;
}

export namespace StreamRessourceResponse {
  export type AsObject = {
    userid: string;
    status: Status;
    dataMap: Array<[string, string]>;
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
