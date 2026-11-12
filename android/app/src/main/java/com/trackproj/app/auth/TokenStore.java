package com.trackproj.app.auth;

import android.content.Context;
import android.content.SharedPreferences;

/**
 * Small SharedPreferences wrapper that keeps the long-lived device token
 * issued by POST /v1/enroll. Java replacement for the Kotlin TokenStore.
 */
public class TokenStore {

    private static final String PREFS = "trackproj_auth";

    private final SharedPreferences prefs;

    public TokenStore(Context context) {
        prefs = context.getApplicationContext()
                .getSharedPreferences(PREFS, Context.MODE_PRIVATE);
    }

    public String getDeviceToken() {
        return prefs.getString("device_token", null);
    }

    public void setDeviceToken(String value) {
        prefs.edit().putString("device_token", value).apply();
    }

    public String getDeviceId() {
        return prefs.getString("device_id", null);
    }

    public void setDeviceId(String value) {
        prefs.edit().putString("device_id", value).apply();
    }

    public boolean hasDevice() {
        return getDeviceToken() != null;
    }

    public void clear() {
        prefs.edit().clear().apply();
    }
}
