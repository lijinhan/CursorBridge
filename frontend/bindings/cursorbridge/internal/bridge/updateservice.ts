// Manually created binding for UpdateService — wails3 CLI not available.
// Uses $Call.ByName since we don't have the generated hash IDs.

import { Call as $Call, CancellablePromise as $CancellablePromise } from "@wailsio/runtime";

import * as $models from "./models.js";

export function CheckForUpdates(): $CancellablePromise<$models.UpdateState> {
    return $Call.ByName("UpdateService.CheckForUpdates").then(($result: any) => {
        return $models.UpdateState.createFrom($result);
    });
}

export function DismissUpdate(): $CancellablePromise<$models.UpdateState> {
    return $Call.ByName("UpdateService.DismissUpdate").then(($result: any) => {
        return $models.UpdateState.createFrom($result);
    });
}

export function DownloadUpdate(): $CancellablePromise<$models.UpdateState> {
    return $Call.ByName("UpdateService.DownloadUpdate").then(($result: any) => {
        return $models.UpdateState.createFrom($result);
    });
}

export function GetState(): $CancellablePromise<$models.UpdateState> {
    return $Call.ByName("UpdateService.GetState").then(($result: any) => {
        return $models.UpdateState.createFrom($result);
    });
}

export function InstallUpdate(): $CancellablePromise<$models.UpdateState> {
    return $Call.ByName("UpdateService.InstallUpdate").then(($result: any) => {
        return $models.UpdateState.createFrom($result);
    });
}