package com.trackproj.app.auth;

import com.trackproj.app.BuildConfig;

import org.json.JSONObject;

import java.io.IOException;
import java.time.Instant;

import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;

/**
 * Blocking HTTP client for the TrackStack device API. All calls must run off
 * the main thread. The base URL comes from BuildConfig.BASE_URL, which is set
 * at build time with -PbaseUrl=... (defaults to the emulator host 10.0.2.2).
 */
public class ApiClient {

    public static final class DeviceResult {
        public final String id;
        public final String apiToken;

        public DeviceResult(String id, String apiToken) {
            this.id = id;
            this.apiToken = apiToken;
        }
    }

    /** Server rejected the device token (device deleted, token revoked/rotated). */
    public static class DeviceUnauthorizedException extends IOException {
        public DeviceUnauthorizedException(String message) {
            super(message);
        }
    }

    private static final MediaType JSON = MediaType.get("application/json");

    private final OkHttpClient client = new OkHttpClient();
    private final String baseUrl = BuildConfig.BASE_URL;

    /** POST /v1/enroll — public. Returns the device id and its one-time token. */
    public DeviceResult enroll(String name) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("name", name);
            body.put("type", "phone");
        } catch (Exception e) {
            throw new IOException("could not build enroll request", e);
        }

        Request request = new Request.Builder()
                .url(baseUrl + "/v1/enroll")
                .post(RequestBody.create(body.toString(), JSON))
                .build();

        try (Response response = client.newCall(request).execute()) {
            String text = response.body() != null ? response.body().string() : "";
            if (!response.isSuccessful()) {
                throw new IOException("enroll failed: " + text);
            }
            try {
                JSONObject json = new JSONObject(text);
                return new DeviceResult(json.getString("id"), json.getString("api_token"));
            } catch (Exception e) {
                throw new IOException("malformed enroll response: " + text, e);
            }
        }
    }

    /** POST /v1/positions authenticated with the device token. */
    public void postPosition(String deviceToken,
                             double lat,
                             double lon,
                             Double speed,
                             Double heading,
                             Double accuracy,
                             long recordedAtMillis) throws IOException {
        JSONObject json = new JSONObject();
        try {
            json.put("lat", lat);
            json.put("lon", lon);
            json.put("recorded_at", Instant.ofEpochMilli(recordedAtMillis).toString());
            if (speed != null) json.put("speed", speed);
            if (heading != null) json.put("heading", heading);
            if (accuracy != null) json.put("accuracy", accuracy);
        } catch (Exception e) {
            throw new IOException("could not build position request", e);
        }

        Request request = new Request.Builder()
                .url(baseUrl + "/v1/positions")
                .addHeader("Authorization", "Bearer " + deviceToken)
                .post(RequestBody.create(json.toString(), JSON))
                .build();

        try (Response response = client.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                String message = "position upload failed (" + response.code() + "): "
                        + (response.body() != null ? response.body().string() : "");
                // 401/403 = token invalid, 404 = device no longer exists -> re-enroll.
                if (response.code() == 401 || response.code() == 403 || response.code() == 404) {
                    throw new DeviceUnauthorizedException(message);
                }
                throw new IOException(message);
            }
        }
    }

    /** POST /v1/tracking-state — tells the dashboard if tracking is intentional. */
    public void setTrackingState(String deviceToken, boolean active) throws IOException {
        JSONObject json = new JSONObject();
        try {
            json.put("active", active);
        } catch (Exception e) {
            throw new IOException("could not build tracking-state request", e);
        }

        Request request = new Request.Builder()
                .url(baseUrl + "/v1/tracking-state")
                .addHeader("Authorization", "Bearer " + deviceToken)
                .post(RequestBody.create(json.toString(), JSON))
                .build();

        try (Response response = client.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                String message = "tracking-state update failed (" + response.code() + "): "
                        + (response.body() != null ? response.body().string() : "");
                if (response.code() == 401 || response.code() == 403 || response.code() == 404) {
                    throw new DeviceUnauthorizedException(message);
                }
                throw new IOException(message);
            }
        }
    }
}
