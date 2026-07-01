package com.trackproj.app.auth

import android.content.Context
import android.content.SharedPreferences

class TokenStore(context: Context) {
    private val prefs: SharedPreferences =
        context.getSharedPreferences("trackproj_auth", Context.MODE_PRIVATE)

    var accessToken: String?
        get() = prefs.getString("access_token", null)
        set(value) = prefs.edit().putString("access_token", value).apply()

    var refreshToken: String?
        get() = prefs.getString("refresh_token", null)
        set(value) = prefs.edit().putString("refresh_token", value).apply()

    var deviceToken: String?
        get() = prefs.getString("device_token", null)
        set(value) = prefs.edit().putString("device_token", value).apply()

    var deviceId: String?
        get() = prefs.getString("device_id", null)
        set(value) = prefs.edit().putString("device_id", value).apply()

    fun isLoggedIn(): Boolean = accessToken != null
    fun hasDevice(): Boolean = deviceToken != null

    fun clear() = prefs.edit().clear().apply()
}
