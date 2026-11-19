// EduWorld platform Android plugin — Keystore-backed SecureStorage (ADR-0086, F060 T052).
package vn.eduworld.platform

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import org.godotengine.godot.Godot
import org.godotengine.godot.plugin.GodotPlugin
import org.godotengine.godot.plugin.UsedByGodot
import java.io.File
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Seals SecureStorage records with an AES-GCM 256 key generated inside Android Keystore (alias `eduworld.session`,
 * not exportable). Only ciphertext is written, to `noBackupFilesDir/secure/<key>.bin` (excluded from Auto Backup).
 * The record key is bound as GCM associated data. Values are never logged.
 * Return codes follow Godot's `Error` enum so the GDScript adapter can map them.
 */
class KeystoreSecureStorage(godot: Godot) : GodotPlugin(godot) {

    private companion object {
        const val KEY_ALIAS = "eduworld.session"
        const val ANDROID_KEYSTORE = "AndroidKeyStore"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val KEY_SIZE_BITS = 256
        const val TAG_LENGTH_BITS = 128
        const val FORMAT_VERSION: Byte = 1
        const val DIR_NAME = "secure"
        const val FILE_EXTENSION = ".bin"
        val KEY_PATTERN = Regex("^[a-z0-9][a-z0-9._-]{0,63}$")

        // Godot Error values.
        const val OK = 0
        const val FAILED = 1
        const val ERR_UNAVAILABLE = 2
        const val ERR_FILE_NOT_FOUND = 7
        const val ERR_FILE_CANT_WRITE = 15
        const val ERR_FILE_CORRUPT = 16
        const val ERR_CANT_CREATE = 20
        const val ERR_INVALID_PARAMETER = 31
    }

    @Volatile
    private var lastError: Int = OK

    override fun getPluginName(): String = "EduWorldPlatform"

    @UsedByGodot
    fun secureIsAvailable(): Boolean = try {
        storageDir() != null && KeyStore.getInstance(ANDROID_KEYSTORE).apply { load(null) } != null
    } catch (e: Exception) {
        false
    }

    @UsedByGodot
    fun secureLastError(): Int = lastError

    @UsedByGodot
    fun secureSave(key: String, data: ByteArray): Int {
        if (!KEY_PATTERN.matches(key)) return fail(ERR_INVALID_PARAMETER)
        val dir = storageDir() ?: return fail(ERR_UNAVAILABLE)
        return try {
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.ENCRYPT_MODE, getOrCreateKey())
            cipher.updateAAD(key.toByteArray(Charsets.UTF_8))
            val sealed = cipher.doFinal(data)
            val iv = cipher.iv
            val record = ByteArray(2 + iv.size + sealed.size)
            record[0] = FORMAT_VERSION
            record[1] = iv.size.toByte()
            System.arraycopy(iv, 0, record, 2, iv.size)
            System.arraycopy(sealed, 0, record, 2 + iv.size, sealed.size)
            val target = File(dir, key + FILE_EXTENSION)
            val temp = File(dir, key + FILE_EXTENSION + ".tmp")
            temp.writeBytes(record)
            if (!temp.renameTo(target)) {
                temp.delete()
                return fail(ERR_FILE_CANT_WRITE)
            }
            ok()
        } catch (e: Exception) {
            fail(ERR_CANT_CREATE)
        }
    }

    @UsedByGodot
    fun secureLoad(key: String): ByteArray {
        if (!KEY_PATTERN.matches(key)) {
            fail(ERR_INVALID_PARAMETER)
            return ByteArray(0)
        }
        val dir = storageDir()
        if (dir == null) {
            fail(ERR_UNAVAILABLE)
            return ByteArray(0)
        }
        val file = File(dir, key + FILE_EXTENSION)
        if (!file.exists()) {
            fail(ERR_FILE_NOT_FOUND)
            return ByteArray(0)
        }
        return try {
            val record = file.readBytes()
            if (record.size < 2 || record[0] != FORMAT_VERSION) throw IllegalStateException("format")
            val ivSize = record[1].toInt()
            if (record.size <= 2 + ivSize) throw IllegalStateException("size")
            val iv = record.copyOfRange(2, 2 + ivSize)
            val sealed = record.copyOfRange(2 + ivSize, record.size)
            val secretKey = existingKey() ?: throw IllegalStateException("key")
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.DECRYPT_MODE, secretKey, GCMParameterSpec(TAG_LENGTH_BITS, iv))
            cipher.updateAAD(key.toByteArray(Charsets.UTF_8))
            val plain = cipher.doFinal(sealed)
            ok()
            plain
        } catch (e: Exception) {
            // Corrupt file, deleted/invalidated Keystore key or wrong key binding (F060-AC-26).
            fail(ERR_FILE_CORRUPT)
            ByteArray(0)
        }
    }

    @UsedByGodot
    fun secureErase(key: String): Int {
        if (!KEY_PATTERN.matches(key)) return fail(ERR_INVALID_PARAMETER)
        val dir = storageDir() ?: return ok()
        var result = OK
        for (name in listOf(key + FILE_EXTENSION, key + FILE_EXTENSION + ".tmp")) {
            val file = File(dir, name)
            if (file.exists() && !file.delete()) result = ERR_FILE_CANT_WRITE
        }
        return if (result == OK) ok() else fail(result)
    }

    @UsedByGodot
    fun secureEraseAll(): Int {
        val dir = storageDir() ?: return ok()
        var result = OK
        dir.listFiles()?.forEach { if (!it.delete()) result = ERR_FILE_CANT_WRITE }
        return if (result == OK) ok() else fail(result)
    }

    private fun storageDir(): File? {
        val context = activity?.applicationContext ?: return null
        val dir = File(context.noBackupFilesDir, DIR_NAME)
        if (!dir.exists() && !dir.mkdirs()) return null
        return dir
    }

    private fun existingKey(): SecretKey? {
        val keyStore = KeyStore.getInstance(ANDROID_KEYSTORE).apply { load(null) }
        return (keyStore.getEntry(KEY_ALIAS, null) as? KeyStore.SecretKeyEntry)?.secretKey
    }

    private fun getOrCreateKey(): SecretKey {
        existingKey()?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEYSTORE)
        generator.init(
            KeyGenParameterSpec.Builder(KEY_ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(KEY_SIZE_BITS)
                .build()
        )
        return generator.generateKey()
    }

    private fun ok(): Int {
        lastError = OK
        return OK
    }

    private fun fail(code: Int): Int {
        lastError = code
        return code
    }
}
