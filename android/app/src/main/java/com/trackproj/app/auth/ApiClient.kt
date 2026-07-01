package com.trackproj.app.auth

import com.trackproj.app.BuildConfig

import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.io.IOException
import java.time.Instant
import java.time.format.DateTimeFormatter

class ApiClient(private val baseUrl: String = BuildConfig.BASE_URL) {

    private val client = OkHttpClient()
    private val jsonMedia = "application/json".toMediaType()

    data class DeviceResult(val id: String, val apiToken: String)

    /** Server rejected the device token (device deleted, token revoked/rotated).
     *  Callers should drop the stored token and re-enroll. */
    class DeviceUnauthorizedException(message: String) : IOException(message)

    /**
     * POST /v1/enroll — public, no login. A freshly installed app enrolls
     * itself and receives a long-lived device token (shown once). The device
     * starts unassigned; the admin later assigns it to an organization.
     */
    fun enroll(name: String, type: String = "phone"): DeviceResult {
        val body = JSONObject()
            .put("name", name)
            .put("type", type)
            .toString()
            .toRequestBody(jsonMedia)

        val req = Request.Builder()
            .url("$baseUrl/v1/enroll")
            .post(body)
            .build()

        client.newCall(req).execute().use { resp ->
            val text = resp.body?.string() ?: ""
            if (!resp.isSuccessful) throw IOException("enroll failed: $text")
            val json = JSONObject(text)
            return DeviceResult(id = json.getString("id"), apiToken = json.getString("api_token"))
        }
    }

    /**
     * POST /v1/positions authenticated with the device token
     * (Authorization: Bearer dtk_...). Blocking; run off the main thread.
     */
    fun postPosition(
        deviceToken: String,
        lat: Double,
        lon: Double,
        speed: Double? = null,
        heading: Double? = null,
        accuracy: Double? = null,
        battery: Double? = null,
        recordedAt: Instant = Instant.now()
    ) {
        val json = JSONObject()
            .put("lat", lat)
            .put("lon", lon)
            .put("recorded_at", DateTimeFormatter.ISO_INSTANT.format(recordedAt))
        if (speed != null) json.put("speed", speed)
        if (heading != null) json.put("heading", heading)
        if (accuracy != null) json.put("accuracy", accuracy)
        if (battery != null) json.put("battery", battery)

        val req = Request.Builder()
            .url("$baseUrl/v1/positions")
            .addHeader("Authorization", "Bearer $deviceToken")
            .post(json.toString().toRequestBody(jsonMedia))
            .build()

        client.newCall(req).execute().use { resp ->
            if (!resp.isSuccessful) {
                val msg = "position upload failed (${resp.code}): ${resp.body?.string()}"
                // 401/403 = token invalid, 404 = device no longer exists → re-enroll.
                if (resp.code == 401 || resp.code == 403 || resp.code == 404)
                    throw DeviceUnauthorizedException(msg)
                throw IOException(msg)
            }
        }
    }
}
