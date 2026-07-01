plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.trackproj.app"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.trackproj.app"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "0.1"

        // Dev backend on the LAN — swap here (or via -P) to retarget the app.
        buildConfigField("String", "BASE_URL", "\"http://192.168.5.198:8080\"")
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

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    composeOptions {
        kotlinCompilerExtensionVersion = "1.5.14"
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.13.1")

    // Jetpack Compose (BOM-managed versions)
    implementation(platform("androidx.compose:compose-bom:2024.06.00"))
    implementation("androidx.activity:activity-compose:1.9.0")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-graphics")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.3")

    // Material Components (supplies the Material3 XML window theme)
    implementation("com.google.android.material:material:1.12.0")

    // Tracking + networking
    implementation("androidx.lifecycle:lifecycle-service:2.8.3")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
    implementation("com.google.android.gms:play-services-location:21.3.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("org.osmdroid:osmdroid-android:6.1.20")
}
