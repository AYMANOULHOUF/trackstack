plugins {
    id("com.android.application")
}

// Defaults to the host machine as seen from the Android emulator. Override for a
// physical device with: ./gradlew assembleDebug -PbaseUrl=http://<lan-ip>:8080
val baseUrl = (project.findProperty("baseUrl") as String?) ?: "http://10.0.2.2:8080"

android {
    namespace = "com.trackproj.app"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.trackproj.app"
        minSdk = 26
        targetSdk = 34
        versionCode = 2
        versionName = "0.2"

        buildConfigField("String", "BASE_URL", "\"$baseUrl\"")
    }

    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        buildConfig = true
    }
}

dependencies {
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("androidx.activity:activity:1.9.0")
    implementation("com.google.android.material:material:1.12.0")

    // Tracking + networking
    implementation("com.google.android.gms:play-services-location:21.3.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("org.osmdroid:osmdroid-android:6.1.20")
}
