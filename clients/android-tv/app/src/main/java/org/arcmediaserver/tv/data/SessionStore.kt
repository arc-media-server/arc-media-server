package org.arcmediaserver.tv.data

import android.content.Context
import android.content.SharedPreferences
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** Encrypts the device token at rest. */
interface SecretBox {
    fun seal(plain: String): String
    fun open(sealed: String): String?
}

/**
 * The server this TV uses and its device token. The token is encrypted with a key kept in the
 * Android Keystore (it never leaves the secure hardware where there is some), so a copy of the
 * app's files is no use without the device.
 */
class SessionStore(private val prefs: SharedPreferences, private val box: SecretBox) {
    var server: HttpUrl?
        get() = prefs.getString(KEY_SERVER, null)?.toHttpUrlOrNull()
        set(value) {
            val changed = value?.toString() != prefs.getString(KEY_SERVER, null)
            prefs.edit().apply {
                if (value == null) remove(KEY_SERVER) else putString(KEY_SERVER, value.toString())
                // A token belongs to the server that gave it.
                if (changed) remove(KEY_TOKEN)
            }.apply()
            if (changed) {
                cached = null
                loaded = true
            }
        }

    // Decrypted once and kept in memory: every request and every picture needs it.
    @Volatile private var cached: String? = null
    @Volatile private var loaded = false

    var token: String?
        get() {
            if (!loaded) {
                cached = prefs.getString(KEY_TOKEN, null)?.let { box.open(it) }
                loaded = true
            }
            return cached
        }
        set(value) {
            cached = value
            loaded = true
            prefs.edit().apply {
                if (value == null) remove(KEY_TOKEN) else putString(KEY_TOKEN, box.seal(value))
            }.apply()
        }

    private companion object {
        const val KEY_SERVER = "server"
        const val KEY_TOKEN = "token"
    }
}

/** AES-GCM with a non-exportable Keystore key. */
class KeystoreSecretBox : SecretBox {
    private fun key(): SecretKey {
        val ks = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (ks.getKey(ALIAS, null) as? SecretKey)?.let { return it }
        val gen = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        gen.init(
            KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build(),
        )
        return gen.generateKey()
    }

    override fun seal(plain: String): String {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key())
        val out = cipher.iv + cipher.doFinal(plain.toByteArray(Charsets.UTF_8))
        return Base64.encodeToString(out, Base64.NO_WRAP)
    }

    override fun open(sealed: String): String? = try {
        val bytes = Base64.decode(sealed, Base64.NO_WRAP)
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes, 0, IV_BYTES))
        String(cipher.doFinal(bytes, IV_BYTES, bytes.size - IV_BYTES), Charsets.UTF_8)
    } catch (e: Exception) {
        // The key went (e.g. the device's lock screen was reset): sign in again.
        ArcLog.Android.warn("stored sign-in can't be read; signing in again (${e.javaClass.simpleName})")
        null
    }

    private companion object {
        const val ALIAS = "arc-device-token"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val IV_BYTES = 12
    }
}

fun sessionStore(context: Context): SessionStore =
    SessionStore(context.getSharedPreferences("arc", Context.MODE_PRIVATE), KeystoreSecretBox())
