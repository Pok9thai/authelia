import React, { useState, useEffect, useCallback } from "react";

import { makeStyles, Typography, Button } from "@material-ui/core";
import { useLocation, useNavigate } from "react-router-dom";
import u2fApi from "u2f-api";

import FingerTouchIcon from "@components/FingerTouchIcon";
import { useNotifications } from "@hooks/NotificationsContext";
import LoginLayout from "@layouts/LoginLayout";
import { FirstFactorPath } from "@services/Api";
import { completeU2FRegistrationProcessStep1, completeU2FRegistrationProcessStep2 } from "@services/RegisterDevice";
import { extractIdentityToken } from "@utils/IdentityToken";

const RegisterSecurityKey = function () {
    const style = useStyles();
    const navigate = useNavigate();
    const location = useLocation();
    const { createErrorNotification } = useNotifications();
    const [, setRegistrationInProgress] = useState(false);

    const processToken = extractIdentityToken(location.search);

    const handleBackClick = () => {
        navigate(FirstFactorPath);
    };

    const registerStep1 = useCallback(async () => {
        if (!processToken) {
            return;
        }
        try {
            setRegistrationInProgress(true);
            const res = await completeU2FRegistrationProcessStep1(processToken);
            const registerRequests: u2fApi.RegisterRequest[] = [];
            for (var i in res.registerRequests) {
                const r = res.registerRequests[i];
                registerRequests.push({
                    appId: res.appId,
                    challenge: r.challenge,
                    version: r.version,
                });
            }
            const registerResponse = await u2fApi.register(registerRequests, [], 60);
            await completeU2FRegistrationProcessStep2(registerResponse);
            setRegistrationInProgress(false);
            navigate(FirstFactorPath);
        } catch (err) {
            console.error(err);
            if ((err as Error).message.includes("Request failed with status code 403")) {
                createErrorNotification(
                    "You must open the link from the same device and browser that initiated the registration process",
                );
            } else {
                createErrorNotification(
                    "Failed to register your security key. The identity verification process might have timed out.",
                );
            }
        }
    }, [processToken, createErrorNotification, navigate]);

    useEffect(() => {
        registerStep1();
    }, [registerStep1]);

    return (
        <LoginLayout title="Touch Security Key">
            <div className={style.icon}>
                <FingerTouchIcon size={64} animated />
            </div>
            <Typography className={style.instruction}>Touch the token on your security key</Typography>
            <Button color="primary" onClick={handleBackClick}>
                Retry
            </Button>
            <Button color="primary" onClick={handleBackClick}>
                Cancel
            </Button>
        </LoginLayout>
    );
};

export default RegisterSecurityKey;

const useStyles = makeStyles((theme) => ({
    icon: {
        paddingTop: theme.spacing(4),
        paddingBottom: theme.spacing(4),
    },
    instruction: {
        paddingBottom: theme.spacing(4),
    },
}));
